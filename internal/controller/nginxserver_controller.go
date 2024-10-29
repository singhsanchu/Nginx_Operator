package controller

import (
	"context"
	webserverv1alpha1 "github.com/example/nginx-operator/api/v1alpha1"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
    "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/manager"
)

// NginxServerReconciler reconciles an NginxServer object
type NginxServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}


//+kubebuilder:rbac:groups=webserver.example.com,resources=nginxservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webserver.example.com,resources=nginxservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;create;update;patch;delete

func (r *NginxServerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the NginxServer instance
	nginxServer := &webserverv1alpha1.NginxServer{}
	err := r.Client.Get(ctx, req.NamespacedName, nginxServer)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("NginxServer resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		log.Error(err, "Failed to get NginxServer.")
		return reconcile.Result{}, err
	}

	// Ensure ConfigMap exists
	configMap := r.createIndexConfigMap(nginxServer)
	if err := r.Client.Create(ctx, configMap); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Failed to create ConfigMap for NginxServer.")
		return reconcile.Result{}, err
	}

	// Ensure PVC exists if persistent is enabled
	if nginxServer.Spec.PvcEnabled {
		pvc := r.createPersistentVolumeClaim(nginxServer)
		if err := r.Client.Create(ctx, pvc); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create PVC for NginxServer.")
			return reconcile.Result{}, err
		}
	} else {
     		// Delete PVC if Persistent is false
     		pvc := &corev1.PersistentVolumeClaim{
     			ObjectMeta: metav1.ObjectMeta{
     				Name:      "nginx-html-pvc",
     				Namespace: nginxServer.Namespace,
     			},
     		}
     		if err := r.Client.Delete(ctx, pvc); err != nil {
     			if !errors.IsNotFound(err) {
     				log.Error(err, "Failed to delete PVC for NginxServer.")
     				return reconcile.Result{}, err
     			}
     		} else {
     			log.Info("PVC deleted for NginxServer", "pvcName", pvc.Name, "namespace", pvc.Namespace)
     		}
     	}


	deployment, err := r.createNginxDeployment(ctx, nginxServer)
    if err := r.Client.Create(ctx, deployment); err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create Deployment for NginxServer.")
			return reconcile.Result{}, err
		}

// Fetch the current Deployment and compare replica count
currentDeployment := &appsv1.Deployment{}
err = r.Client.Get(ctx, client.ObjectKey{Name: nginxServer.Name, Namespace: nginxServer.Namespace}, currentDeployment)
if err != nil {
    log.Error(err, "Failed to get current Deployment for NginxServer.")
    return reconcile.Result{}, err
}

// Update replica count if it doesn't match
desiredReplicas := nginxServer.Spec.Replica
if *currentDeployment.Spec.Replicas != desiredReplicas {
    currentDeployment.Spec.Replicas = &desiredReplicas
    if err := r.Client.Update(ctx, currentDeployment); err != nil {
        log.Error(err, "Failed to update Deployment replica count.")
        return reconcile.Result{}, err
    }
    log.Info("Updated Deployment replica count", "replica", desiredReplicas)
}
// Ensure Service exists to expose NGINX
	service, err := r.createNginxService(ctx, nginxServer)
	if err != nil {
		log.Error(err, "Failed to create Service for NginxServer.")
		return reconcile.Result{}, err
	}
	log.Info("NGINX Service created", "serviceName", service.Name, "namespace", service.Namespace)

	return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
}

// createIndexConfigMap creates a ConfigMap for the Nginx index.html file
func (r *NginxServerReconciler) createIndexConfigMap(nginxServer *webserverv1alpha1.NginxServer) *corev1.ConfigMap {
	labels := map[string]string{"app": "nginx", "nginxserver_cr": nginxServer.Name}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-index-html",
			Namespace: nginxServer.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"index.html": "<html><body><h1>Hello World</h1></body></html>",
		},
	}
}

func (r *NginxServerReconciler) createPersistentVolumeClaim(nginxServer *webserverv1alpha1.NginxServer) *corev1.PersistentVolumeClaim {
    labels := map[string]string{"app": "nginx", "nginxserver_cr": nginxServer.Name}
    size := nginxServer.Spec.Size

    return &corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "nginx-html-pvc",
            Namespace: nginxServer.Namespace,
            Labels:    labels,
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
            Resources: corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: resource.MustParse(size),
                },
            },
        },
    }
}

// createNginxService creates a Service to expose the Nginx server
func (r *NginxServerReconciler) createNginxService(ctx context.Context, nginxServer *webserverv1alpha1.NginxServer) (*corev1.Service, error) {
	labels := map[string]string{"app": "nginx", "nginxserver_cr": nginxServer.Name}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-service",
			Namespace: nginxServer.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
                    TargetPort: intstr.FromInt(80),
                    Protocol:   corev1.ProtocolTCP,
                    NodePort: 30070,
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}
	if err := controllerutil.SetControllerReference(nginxServer, service, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(ctx, service)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}
	return service, nil
}


// createNginxDeployment creates the Nginx Deployment with optional PVC and Init Container for index.html
func (r *NginxServerReconciler) createNginxDeployment(ctx context.Context, nginxServer *webserverv1alpha1.NginxServer) (*appsv1.Deployment, error) {
	labels := map[string]string{"app": "nginx", "nginxserver_cr": nginxServer.Name}
	pvcEnabled := nginxServer.Spec.PvcEnabled
    replicas := nginxServer.Spec.Replica

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginxServer.Name,
			Namespace: nginxServer.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
		    Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80, Name: "http"},
							},
						},
					},
				},
			},
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "index-html-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "nginx-index-html"},
				},
			},
		},
	}

	if pvcEnabled {
		volumes = append(volumes, corev1.Volume{
			Name: "nginx-html-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "nginx-html-pvc",
				},
			},
		})

		initContainer := corev1.Container{
			Name:    "copy-index",
			Image:   "busybox",
			Command: []string{"sh", "-c", "cp /config/index.html /usr/share/nginx/html/index.html"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "index-html-config", MountPath: "/config"},
				{Name: "nginx-html-pvc", MountPath: "/usr/share/nginx/html"},
			},
		}
		deployment.Spec.Template.Spec.InitContainers = []corev1.Container{initContainer}

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{Name: "nginx-html-pvc", MountPath: "/usr/share/nginx/html"},
		}
	} else {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{Name: "index-html-config", MountPath: "/usr/share/nginx/html"},
		}
	}

	deployment.Spec.Template.Spec.Volumes = volumes

	if err := controllerutil.SetControllerReference(nginxServer, deployment, r.Scheme); err != nil {
		return nil, err
	}

	if err := r.Client.Create(ctx, deployment); err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	return deployment, nil
}


func (r *NginxServerReconciler) SetupWithManager(mgr manager.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&webserverv1alpha1.NginxServer{}).
        Complete(r)
}