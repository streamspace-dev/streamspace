package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
	"github.com/streamspace/streamspace/pkg/metrics"
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
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the main reconciliation loop
func (r *SessionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	startTime := time.Now()

	// Track reconciliation metrics
	defer func() {
		duration := time.Since(startTime).Seconds()
		metrics.ObserveReconciliationDuration(req.Namespace, duration)
	}()

	// Fetch the Session
	var session streamv1alpha1.Session
	if err := r.Get(ctx, req.NamespacedName, &session); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Session resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Session")
		metrics.RecordReconciliation(req.Namespace, "error")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Session", "name", session.Name, "state", session.Spec.State)

	// Update metrics for this session
	metrics.RecordSessionByUser(session.Spec.User, session.Namespace, 1)
	metrics.RecordSessionByTemplate(session.Spec.Template, session.Namespace, 1)

	// Get the Template
	template, err := r.getTemplate(ctx, session.Spec.Template, session.Namespace)
	if err != nil {
		log.Error(err, "Failed to get Template")
		metrics.RecordReconciliation(req.Namespace, "error")
		return ctrl.Result{}, err
	}

	// Handle state transitions
	var result ctrl.Result
	switch session.Spec.State {
	case "running":
		result, err = r.handleRunning(ctx, &session, template)
	case "hibernated":
		result, err = r.handleHibernated(ctx, &session)
	case "terminated":
		result, err = r.handleTerminated(ctx, &session)
	default:
		log.Info("Unknown state", "state", session.Spec.State)
		return ctrl.Result{}, nil
	}

	// Record reconciliation result
	if err != nil {
		metrics.RecordReconciliation(req.Namespace, "error")
	} else {
		metrics.RecordReconciliation(req.Namespace, "success")
	}

	return result, err
}

func (r *SessionReconciler) handleRunning(ctx context.Context, session *streamv1alpha1.Session, template *streamv1alpha1.Template) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	serviceName := fmt.Sprintf("%s-svc", deploymentName)

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

	// Ensure Service exists
	service := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: session.Namespace}, service)

	if errors.IsNotFound(err) {
		// Create new service
		service = r.createService(session, template)
		if err := r.Create(ctx, service); err != nil {
			log.Error(err, "Failed to create Service")
			return ctrl.Result{}, err
		}
		log.Info("Created Service", "name", serviceName)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure user PVC exists if persistent home is enabled
	if session.Spec.PersistentHome {
		pvcName := fmt.Sprintf("home-%s", session.Spec.User)
		pvc := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: session.Namespace}, pvc)

		if errors.IsNotFound(err) {
			// Create new PVC
			pvc = r.createUserPVC(session)
			if err := r.Create(ctx, pvc); err != nil {
				log.Error(err, "Failed to create PVC")
				return ctrl.Result{}, err
			}
			log.Info("Created user PVC", "name", pvcName)
		} else if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Ensure Ingress exists
	ingressName := deploymentName
	ingress := &networkingv1.Ingress{}
	err = r.Get(ctx, types.NamespacedName{Name: ingressName, Namespace: session.Namespace}, ingress)

	if errors.IsNotFound(err) {
		// Create new ingress
		ingress = r.createIngress(session, template, serviceName)
		if err := r.Create(ctx, ingress); err != nil {
			log.Error(err, "Failed to create Ingress")
			return ctrl.Result{}, err
		}
		log.Info("Created Ingress", "name", ingressName)
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Get ingress domain for URL
	ingressDomain := os.Getenv("INGRESS_DOMAIN")
	if ingressDomain == "" {
		ingressDomain = "streamspace.local"
	}

	// Update Session status
	session.Status.Phase = "Running"
	session.Status.PodName = deploymentName
	session.Status.URL = fmt.Sprintf("https://%s.%s", session.Name, ingressDomain)
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

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Add user home volume if persistent home is enabled
	if session.Spec.PersistentHome {
		pvcName := fmt.Sprintf("home-%s", session.Spec.User)

		// Add volume mount to container
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "user-home",
			MountPath: "/config",
		})

		// Add volume to pod spec
		podSpec.Volumes = []corev1.Volume{
			{
				Name: "user-home",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		}
	}

	// Update pod spec with modified container
	podSpec.Containers[0] = container

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
				Spec: podSpec,
			},
		},
	}

	return deployment
}

func (r *SessionReconciler) createService(session *streamv1alpha1.Session, template *streamv1alpha1.Template) *corev1.Service {
	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	serviceName := fmt.Sprintf("%s-svc", deploymentName)
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Determine VNC port
	vncPort := int32(5900)
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: session.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "vnc",
					Port:     vncPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

func (r *SessionReconciler) createUserPVC(session *streamv1alpha1.Session) *corev1.PersistentVolumeClaim {
	pvcName := fmt.Sprintf("home-%s", session.Spec.User)
	labels := map[string]string{
		"app":  "streamspace-user-home",
		"user": session.Spec.User,
	}

	// Default home directory size
	storageSize := "50Gi"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: session.Namespace,
			Labels:    labels,
			// Note: No owner reference - PVC persists across sessions
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany, // NFS support
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}

	return pvc
}

func (r *SessionReconciler) createIngress(session *streamv1alpha1.Session, template *streamv1alpha1.Template, serviceName string) *networkingv1.Ingress {
	deploymentName := fmt.Sprintf("ss-%s-%s", session.Spec.User, session.Spec.Template)
	labels := map[string]string{
		"app":      "streamspace-session",
		"user":     session.Spec.User,
		"template": session.Spec.Template,
		"session":  session.Name,
	}

	// Get ingress configuration from environment
	ingressDomain := os.Getenv("INGRESS_DOMAIN")
	if ingressDomain == "" {
		ingressDomain = "streamspace.local"
	}

	ingressClass := os.Getenv("INGRESS_CLASS")
	if ingressClass == "" {
		ingressClass = "traefik"
	}

	// Determine VNC port
	vncPort := int32(5900)
	if template.Spec.VNC.Port != 0 {
		vncPort = int32(template.Spec.VNC.Port)
	}

	// Build hostname
	hostname := fmt.Sprintf("%s.%s", session.Name, ingressDomain)

	// Path type
	pathTypePrefix := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: session.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": ingressClass,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(session, streamv1alpha1.GroupVersion.WithKind("Session")),
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClass,
			Rules: []networkingv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: vncPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return ingress
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
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func int32Ptr(i int32) *int32 { return &i }
