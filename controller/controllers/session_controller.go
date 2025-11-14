package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
)

// SessionReconciler reconciles a Session object
type SessionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=sessions/finalizers,verbs=update
//+kubebuilder:rbac:groups=stream.streamspace.io,resources=templates,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop
func (r *SessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Session
	var session streamv1alpha1.Session
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Session resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Session")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Session", "name", session.Name, "state", session.Spec.State)

	// Get the Template
	template, err := r.getTemplate(ctx, session.Spec.Template, session.Namespace)
	if err != nil {
		log.Error(err, "Failed to get Template")
		return ctrl.Result{}, err
	}

	// Handle state transitions
	switch session.Spec.State {
	case "running":
		return r.handleRunning(ctx, &session, template)
	case "hibernated":
		return r.handleHibernated(ctx, &session)
	case "terminated":
		return r.handleTerminated(ctx, &session)
	default:
		log.Info("Unknown state", "state", session.Spec.State)
		return ctrl.Result{}, nil
	}
}

func (r *SessionReconciler) handleRunning(ctx context.Context, session *streamv1alpha1.Session, template *streamv1alpha1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Check if deployment exists
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if errors.IsNotFound(err) {
		// Create new deployment
		deployment = r.createDeployment(session, template)
		if err := r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Created Deployment", "name", deploymentName)
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		// Deployment exists, ensure it's running
		if *deployment.Spec.Replicas == 0 {
			deployment.Spec.Replicas = int32Ptr(1)
			if err := r.Update(ctx, deployment); err != nil {
				log.Error(err, "Failed to scale up Deployment")
				return ctrl.Result{}, err
			}
			log.Info("Scaled up Deployment", "name", deploymentName)
		}
	}

	// Update Session status
	session.Status.Phase = "Running"
	session.Status.PodName = deploymentName
	session.Status.URL = fmt.Sprintf("https://%s.streamspace.local", session.Name)
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SessionReconciler) handleHibernated(ctx context.Context, session *streamv1alpha1.Session) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Scale deployment to 0
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if err == nil && *deployment.Spec.Replicas > 0 {
		deployment.Spec.Replicas = int32Ptr(0)
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed to scale down Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Scaled down Deployment (hibernated)", "name", deploymentName)
	}

	// Update Session status
	session.Status.Phase = "Hibernated"
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SessionReconciler) handleTerminated(ctx context.Context, session *streamv1alpha1.Session) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)

	// Delete deployment
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: session.Namespace}, deployment)

	if err == nil {
		if err := r.Delete(ctx, deployment); err != nil {
			log.Error(err, "Failed to delete Deployment")
			return ctrl.Result{}, err
		}
		log.Info("Deleted Deployment (terminated)", "name", deploymentName)
	}

	// Update Session status
	session.Status.Phase = "Terminated"
	if err := r.Status().Update(ctx, session); err != nil {
		log.Error(err, "Failed to update Session status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SessionReconciler) createDeployment(session *streamv1alpha1.Session, template *streamv1alpha1.Template) *appsv1.Deployment {
	name := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Determine VNC port (use template's VNC config or default)
	vncPort := int32(5900) // Standard VNC port
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	// Build container
	container := corev1.Container{
		Name:  "session",
		Image: template.Spec.BaseImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          "vnc",
				ContainerPort: vncPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: template.Spec.Env,
	}

	// Add resources if specified
	if len(session.Spec.Resources.Requests) > 0 || len(session.Spec.Resources.Limits) > 0 {
		container.Resources = session.Spec.Resources
	} else if len(template.Spec.DefaultResources.Requests) > 0 || len(template.Spec.DefaultResources.Limits) > 0 {
		container.Resources = template.Spec.DefaultResources
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: session.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	return deployment
}

func (r *SessionReconciler) getTemplate(ctx context.Context, templateName, namespace string) (*streamv1alpha1.Template, error) {
	template := &streamv1alpha1.Template{}
	err := r.Get(ctx, types.NamespacedName{Name: templateName, Namespace: namespace}, template)
	if err != nil {
		return nil, err
	}
	return template, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SessionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&streamv1alpha1.Session{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func int32Ptr(i int32) *int32 { return &i }
