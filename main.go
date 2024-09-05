package main

import (
	"flag"
	"github.com/Sumon009838/CRD/controller"
	clientset "github.com/Sumon009838/CRD/pkg/generated/clientset/versioned"
	informers "github.com/Sumon009838/CRD/pkg/generated/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"path/filepath"
	"time"
)

func main() {
	log.Println("Configure KubeConfig")
	var kubeConfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = flag.String("kubeConfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeConfig file")
	} else {
		kubeConfig = flag.String("kubeConfig", "", "absolute path to the kubeConfig file")
	}
	config, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		panic(err.Error())
	}
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	exampleClient, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	kubeInformationFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	c := controller.NewController(kubeClient, exampleClient, kubeInformationFactory.Apps().V1().Deployments(), exampleInformerFactory.BookStore().V1().BookStores())
	stopCh := make(chan struct{})
	kubeInformationFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err := c.Run(1, stopCh); err != nil {
		log.Println("Error running controller:", err)
	}
}
