package controller

import (
	"context"
	"fmt"
	controllerv1 "github.com/Sumon009838/CRD/pkg/apis/reader.com/v1"
	clientset "github.com/Sumon009838/CRD/pkg/generated/clientset/versioned"
	informer "github.com/Sumon009838/CRD/pkg/generated/informers/externalversions/reader.com/v1"
	lister "github.com/Sumon009838/CRD/pkg/generated/listers/reader.com/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsInformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appsListers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

type Controller struct {
	KubeClientSet   kubernetes.Interface
	SampleClientSet clientset.Interface

	DeploymentsLister appsListers.DeploymentLister
	DeploymentsSynced cache.InformerSynced
	BookStoreLister   lister.BookStoreLister
	BookStoreSynced   cache.InformerSynced
	WorkQueue         workqueue.RateLimitingInterface
}

func NewController(
	kubectlclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	deploymentInformer appsInformer.DeploymentInformer,
	bookstoreInformer informer.BookStoreInformer) *Controller {
	cntrlr := &Controller{
		KubeClientSet:     kubectlclientset,
		SampleClientSet:   sampleclientset,
		DeploymentsLister: deploymentInformer.Lister(),
		DeploymentsSynced: deploymentInformer.Informer().HasSynced,
		BookStoreLister:   bookstoreInformer.Lister(),
		BookStoreSynced:   bookstoreInformer.Informer().HasSynced,
		WorkQueue:         workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
	log.Println("Setting up event handlers")
	bookstoreInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: cntrlr.enqueueBookStore,
		UpdateFunc: func(oldObj, newObj interface{}) {
			cntrlr.enqueueBookStore(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			cntrlr.enqueueBookStore(obj)
		},
	})
	return cntrlr
}
func (c *Controller) enqueueBookStore(obj interface{}) {
	log.Println("Enqueuing book store")
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.WorkQueue.AddRateLimited(key)
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.WorkQueue.ShutDown()
	log.Println("Starting BookStore controller...")
	log.Println("Waiting for informer caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, c.DeploymentsSynced, c.BookStoreSynced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")
	}
	log.Println("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	log.Println("Worker started successfully")
	<-stopCh
	log.Println("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.ProcessNextItem() {
		time.Sleep(2 * time.Second)
	}

}
func (c *Controller) ProcessNextItem() bool {
	obj, shutdown := c.WorkQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.WorkQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.WorkQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.WorkQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.WorkQueue.Forget(obj)
		log.Printf("Successfully synced '%s'", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	bookStore, err := c.BookStoreLister.BookStores(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("BookStore '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	deploymentName := bookStore.Spec.Name
	if deploymentName == "" {
		utilruntime.HandleError(fmt.Errorf("%s : deployment name must be specified", key))
		return nil
	}

	deployment, err := c.DeploymentsLister.Deployments(namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.KubeClientSet.AppsV1().Deployments(bookStore.Namespace).Create(context.TODO(), newDeployment(bookStore), metav1.CreateOptions{})
	}
	if err != nil {
		return err
	}
	err = c.updateBookStoreStatus(bookStore)
	if err != nil {
		return err
	}
	if bookStore.Spec.Replicas != nil && *bookStore.Spec.Replicas != *deployment.Spec.Replicas {
		log.Printf("BookStore in %s has  replicas : %d , deployment has replicas: %d\n", namespace, *bookStore.Spec.Replicas, *deployment.Spec.Replicas)

		deployment, err = c.KubeClientSet.AppsV1().Deployments(namespace).Update(context.TODO(), newDeployment(bookStore), metav1.UpdateOptions{})
		if err != nil {
			return err
		}

	}

	serviceName := bookStore.Spec.Name + "-service"
	service, err := c.KubeClientSet.CoreV1().Services(bookStore.Namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})

	if errors.IsNotFound(err) {
		service, err = c.KubeClientSet.CoreV1().Services(bookStore.Namespace).Create(context.TODO(), newService(bookStore), metav1.CreateOptions{})
		if err != nil {
			return err
		}
		log.Printf("\nservice %s is created", service.Name)
	} else if err != nil {
		return err
	}
	_, err = c.KubeClientSet.CoreV1().Services(bookStore.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) updateBookStoreStatus(bookStore *controllerv1.BookStore) error {
	bookStoreCopy := bookStore.DeepCopy()
	y := int32(rand.Intn(11) + 1)
	bookStoreCopy.Spec.Replicas = &y
	fmt.Println("y: ", y)

	_, err := c.SampleClientSet.BookStoreV1().BookStores(bookStore.Namespace).Update(context.TODO(), bookStoreCopy, metav1.UpdateOptions{})
	return err
}
func newDeployment(bookStore *controllerv1.BookStore) *appsv1.Deployment {
	fmt.Println("Inside newDeployment +++++++++++")
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bookStore.Spec.Name,
			Namespace: bookStore.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: bookStore.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "bookstore-app",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "bookstore-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "bookstore-app",
							Image: bookStore.Spec.Container.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: bookStore.Spec.Container.Port,
								},
							},
						},
					},
				},
			},
		},
	}

}
func newService(bookStore *controllerv1.BookStore) *corev1.Service {
	fmt.Println("---+++--- ", bookStore.Spec.Container.Port)
	labels := map[string]string{
		"app": "bookstore-app",
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bookStore.Spec.Name + "-service",
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(bookStore, controllerv1.SchemeGroupVersion.WithKind("BookStore")),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       3200,
					TargetPort: intstr.FromInt32(bookStore.Spec.Container.Port),
					NodePort:   30002,
				},
			},
		},
	}
}
