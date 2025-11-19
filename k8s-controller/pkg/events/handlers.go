// Package events provides NATS event handlers for the StreamSpace controller.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// handleSessionCreate handles session creation events.
func (s *Subscriber) handleSessionCreate(ctx context.Context, data []byte) error {
	var event SessionCreateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal SessionCreateEvent: %w", err)
	}

	log.Printf("Handling session create event: %s for user %s", event.SessionID, event.UserID)

	// Create Session CRD
	session := &streamv1alpha1.Session{
		ObjectMeta: metav1.ObjectMeta{
			Name:      event.SessionID,
			Namespace: s.namespace,
			Labels: map[string]string{
				"streamspace.io/user":     event.UserID,
				"streamspace.io/template": event.TemplateID,
			},
		},
		Spec: streamv1alpha1.SessionSpec{
			User:           event.UserID,
			Template:       event.TemplateID,
			State:          "running",
			PersistentHome: event.PersistentHome,
			IdleTimeout:    event.IdleTimeout,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(event.Resources.Memory),
					corev1.ResourceCPU:    resource.MustParse(event.Resources.CPU),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(event.Resources.Memory),
					corev1.ResourceCPU:    resource.MustParse(event.Resources.CPU),
				},
			},
		},
	}

	if err := s.client.Create(ctx, session); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Printf("Session %s already exists", event.SessionID)
		} else {
			s.publishSessionStatus(event.SessionID, "failed", "", fmt.Sprintf("Failed to create session: %v", err))
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	log.Printf("Session %s created successfully", event.SessionID)
	return nil
}

// handleSessionDelete handles session deletion events.
func (s *Subscriber) handleSessionDelete(ctx context.Context, data []byte) error {
	var event SessionDeleteEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal SessionDeleteEvent: %w", err)
	}

	log.Printf("Handling session delete event: %s", event.SessionID)

	// Delete Session CRD
	session := &streamv1alpha1.Session{
		ObjectMeta: metav1.ObjectMeta{
			Name:      event.SessionID,
			Namespace: s.namespace,
		},
	}

	if err := s.client.Delete(ctx, session); err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Session %s already deleted", event.SessionID)
		} else {
			return fmt.Errorf("failed to delete session: %w", err)
		}
	}

	log.Printf("Session %s deleted successfully", event.SessionID)
	return nil
}

// handleSessionHibernate handles session hibernation events.
func (s *Subscriber) handleSessionHibernate(ctx context.Context, data []byte) error {
	var event SessionHibernateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal SessionHibernateEvent: %w", err)
	}

	log.Printf("Handling session hibernate event: %s", event.SessionID)

	// Get the session
	session := &streamv1alpha1.Session{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      event.SessionID,
		Namespace: s.namespace,
	}, session); err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update state to hibernated
	session.Spec.State = "hibernated"
	if err := s.client.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	// Scale deployment to 0
	deploymentName := fmt.Sprintf("ss-%s", event.SessionID)
	deployment := &appsv1.Deployment{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      deploymentName,
		Namespace: s.namespace,
	}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		replicas := int32(0)
		deployment.Spec.Replicas = &replicas
		if err := s.client.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to scale deployment to 0: %w", err)
		}
	}

	s.publishSessionStatus(event.SessionID, "hibernated", "Hibernated", "Session hibernated")
	log.Printf("Session %s hibernated successfully", event.SessionID)
	return nil
}

// handleSessionWake handles session wake events.
func (s *Subscriber) handleSessionWake(ctx context.Context, data []byte) error {
	var event SessionWakeEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal SessionWakeEvent: %w", err)
	}

	log.Printf("Handling session wake event: %s", event.SessionID)

	// Get the session
	session := &streamv1alpha1.Session{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      event.SessionID,
		Namespace: s.namespace,
	}, session); err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Update state to running
	session.Spec.State = "running"
	if err := s.client.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	// Scale deployment to 1
	deploymentName := fmt.Sprintf("ss-%s", event.SessionID)
	deployment := &appsv1.Deployment{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      deploymentName,
		Namespace: s.namespace,
	}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		replicas := int32(1)
		deployment.Spec.Replicas = &replicas
		if err := s.client.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to scale deployment to 1: %w", err)
		}
	}

	s.publishSessionStatus(event.SessionID, "running", "Running", "Session woken")
	log.Printf("Session %s woken successfully", event.SessionID)
	return nil
}

// handleAppInstall handles application installation events.
func (s *Subscriber) handleAppInstall(ctx context.Context, data []byte) error {
	var event AppInstallEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal AppInstallEvent: %w", err)
	}

	log.Printf("Handling app install event: %s (%s)", event.InstallID, event.TemplateName)

	// Create ApplicationInstall CRD
	appInstall := &streamv1alpha1.ApplicationInstall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      event.InstallID,
			Namespace: s.namespace,
			Labels: map[string]string{
				"streamspace.io/template":    event.TemplateName,
				"streamspace.io/category":    event.Category,
				"streamspace.io/installed-by": event.InstalledBy,
			},
		},
		Spec: streamv1alpha1.ApplicationInstallSpec{
			TemplateName:      event.TemplateName,
			DisplayName:       event.DisplayName,
			Description:       event.Description,
			Category:          event.Category,
			Icon:              event.IconURL,
			Manifest:          event.Manifest,
			CatalogTemplateID: event.CatalogTemplateID,
		},
	}

	if err := s.client.Create(ctx, appInstall); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Printf("ApplicationInstall %s already exists", event.InstallID)
			// Publish status as installed since it already exists
			s.publishAppStatus(event.InstallID, "installed", event.TemplateName, "ApplicationInstall already exists")
		} else {
			s.publishAppStatus(event.InstallID, "failed", event.TemplateName, fmt.Sprintf("Failed to create ApplicationInstall: %v", err))
			return fmt.Errorf("failed to create ApplicationInstall: %w", err)
		}
	} else {
		// Successfully created - publish creating status
		// The ApplicationInstallReconciler will update to "installed" when Template is ready
		s.publishAppStatus(event.InstallID, "creating", event.TemplateName, "ApplicationInstall CRD created, creating Template...")
	}

	log.Printf("ApplicationInstall %s created successfully", event.InstallID)
	return nil
}

// handleAppUninstall handles application uninstallation events.
func (s *Subscriber) handleAppUninstall(ctx context.Context, data []byte) error {
	var event AppUninstallEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal AppUninstallEvent: %w", err)
	}

	log.Printf("Handling app uninstall event: %s", event.InstallID)

	// Delete ApplicationInstall CRD (will cascade delete Template due to owner reference)
	appInstall := &streamv1alpha1.ApplicationInstall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      event.InstallID,
			Namespace: s.namespace,
		},
	}

	if err := s.client.Delete(ctx, appInstall); err != nil {
		if errors.IsNotFound(err) {
			log.Printf("ApplicationInstall %s already deleted", event.InstallID)
		} else {
			return fmt.Errorf("failed to delete ApplicationInstall: %w", err)
		}
	}

	log.Printf("ApplicationInstall %s deleted successfully", event.InstallID)
	return nil
}

// handleTemplateCreate handles template creation events.
func (s *Subscriber) handleTemplateCreate(ctx context.Context, data []byte) error {
	var event TemplateCreateEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal TemplateCreateEvent: %w", err)
	}

	log.Printf("Handling template create event: %s", event.TemplateID)
	// Templates are typically created via the API's k8sClient or via ApplicationInstall
	// This handler is for future use when templates are created purely through events
	log.Printf("Template create event received for %s (handled by API)", event.TemplateID)
	return nil
}

// handleTemplateDelete handles template deletion events.
func (s *Subscriber) handleTemplateDelete(ctx context.Context, data []byte) error {
	var event TemplateDeleteEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal TemplateDeleteEvent: %w", err)
	}

	log.Printf("Handling template delete event: %s", event.TemplateID)
	// Templates are typically deleted via the API's k8sClient
	// This handler is for future use when templates are deleted purely through events
	log.Printf("Template delete event received for %s (handled by API)", event.TemplateID)
	return nil
}

// handleNodeCordon handles node cordon events.
func (s *Subscriber) handleNodeCordon(ctx context.Context, data []byte) error {
	var event NodeCordonEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal NodeCordonEvent: %w", err)
	}

	log.Printf("Handling node cordon event: %s", event.NodeName)

	// Get the node
	node := &corev1.Node{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: event.NodeName}, node); err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Set unschedulable
	node.Spec.Unschedulable = true
	if err := s.client.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	log.Printf("Node %s cordoned successfully", event.NodeName)
	return nil
}

// handleNodeUncordon handles node uncordon events.
func (s *Subscriber) handleNodeUncordon(ctx context.Context, data []byte) error {
	var event NodeUncordonEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal NodeUncordonEvent: %w", err)
	}

	log.Printf("Handling node uncordon event: %s", event.NodeName)

	// Get the node
	node := &corev1.Node{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: event.NodeName}, node); err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Clear unschedulable
	node.Spec.Unschedulable = false
	if err := s.client.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to uncordon node: %w", err)
	}

	log.Printf("Node %s uncordoned successfully", event.NodeName)
	return nil
}

// handleNodeDrain handles node drain events.
func (s *Subscriber) handleNodeDrain(ctx context.Context, data []byte) error {
	var event NodeDrainEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal NodeDrainEvent: %w", err)
	}

	log.Printf("Handling node drain event: %s", event.NodeName)

	// First cordon the node
	node := &corev1.Node{}
	if err := s.client.Get(ctx, types.NamespacedName{Name: event.NodeName}, node); err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	node.Spec.Unschedulable = true
	if err := s.client.Update(ctx, node); err != nil {
		return fmt.Errorf("failed to cordon node before drain: %w", err)
	}

	// List pods on the node
	podList := &corev1.PodList{}
	if err := s.client.List(ctx, podList, client.MatchingFields{"spec.nodeName": event.NodeName}); err != nil {
		return fmt.Errorf("failed to list pods on node: %w", err)
	}

	// Delete pods (evict them)
	gracePeriod := int64(30)
	if event.GracePeriodSeconds != nil {
		gracePeriod = *event.GracePeriodSeconds
	}

	for _, pod := range podList.Items {
		// Skip mirror pods and DaemonSet pods
		if pod.Annotations["kubernetes.io/config.mirror"] != "" {
			continue
		}
		if metav1.GetControllerOf(&pod) != nil {
			for _, ref := range pod.OwnerReferences {
				if ref.Kind == "DaemonSet" {
					continue
				}
			}
		}

		// Delete the pod with grace period
		deleteOpts := &client.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}
		if err := s.client.Delete(ctx, &pod, deleteOpts); err != nil {
			if !errors.IsNotFound(err) {
				log.Printf("Failed to evict pod %s: %v", pod.Name, err)
			}
		} else {
			log.Printf("Evicted pod %s from node %s", pod.Name, event.NodeName)
		}
	}

	log.Printf("Node %s drained successfully", event.NodeName)
	return nil
}

// publishSessionStatus publishes a session status update.
func (s *Subscriber) publishSessionStatus(sessionID, status, phase, message string) {
	event := SessionStatusEvent{
		EventID:      uuid.New().String(),
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		Status:       status,
		Phase:        phase,
		Message:      message,
		ControllerID: s.controllerID,
	}

	if err := s.publishStatus(SubjectSessionStatus, event); err != nil {
		log.Printf("Failed to publish session status: %v", err)
	}
}

// publishAppStatus publishes an app installation status update.
func (s *Subscriber) publishAppStatus(installID, status, templateName, message string) {
	event := AppStatusEvent{
		EventID:      uuid.New().String(),
		Timestamp:    time.Now(),
		InstallID:    installID,
		Status:       status,
		TemplateName: templateName,
		Message:      message,
		ControllerID: s.controllerID,
	}

	if err := s.publishStatus(SubjectAppStatus, event); err != nil {
		log.Printf("Failed to publish app status: %v", err)
	}
}
