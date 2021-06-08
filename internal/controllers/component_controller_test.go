package controllers

//
//import (
//	"context"
//	"fmt"
//	"testing"
//
//	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
//)
//
//func Test_reconcile(t *testing.T) {
//	component := &ketchv1.Component{
//		Spec: ketchv1.ComponentSpec{
//			Schematic: ketchv1.Schematic{
//				Kube: &ketchv1.Kube{
//					Templates: []ketchv1.Template{
//						ketchv1.Template(`apiVersion: apps/v1
//kind: Deployment
//spec:
//  selector:
//	matchLabels:
//	  app.ketch.io/component: frontend
//  template:
//	metadata:
//	  labels:
//		app.ketch.io/component: frontend
//	spec:
//	  containers:
//	  - name: frontend
//		ports:
//		- containerPort: 80
//		livenessProbe:
//		  httpGet:
//			path: /
//			port: 80
//		readinessProbe:
//		  httpGet:
//			path: /
//			port: 80`),
//					},
//				},
//			},
//		},
//	}
//	componentReconciler := ComponentReconciler{}
//	componentStatus := componentReconciler.reconcile(context.Background(), component)
//	fmt.Println(componentStatus)
//}
