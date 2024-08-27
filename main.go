package main

import (
	"context"
	"flag"
	myv1 "github.com/Sumon009838/CRD/pkg/apis/reader.com/v1"
	myclient "github.com/Sumon009838/CRD/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"path/filepath"
	"time"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	ctx := context.TODO()
	client, err := myclient.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	bookstoreobj := myv1.BookStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bookstore",
		},
		Spec: myv1.BookStoreSpec{
			Name:     "bookstore",
			Replicas: intPtr(3),
			Container: myv1.ContainerSpec{
				Image: "sumon124816/k8s:new",
				Port:  "8080",
			},
		},
	}
	_, err = client.BookStoreV1().BookStores("default").Create(ctx, &bookstoreobj, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	time.Sleep(5 * time.Second)
	log.Println("BookStore created! ")
}
func intPtr(i int32) *int32 {
	return &i
}
