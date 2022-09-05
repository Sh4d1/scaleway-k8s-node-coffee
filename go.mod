module github.com/Sh4d1/scaleway-k8s-node-coffee

go 1.15

require (
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.9.0.20220905101028-f685ad03ae6c
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009 // indirect
)

replace k8s.io/kubernetes => k8s.io/kubernetes v0.20.1
