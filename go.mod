module github.com/brwallis/srlinux-kubemgr

go 1.16

//replace srlinux.io/go => ../srlinux-go

require (
	github.com/openconfig/gnmi v0.0.0-20210226144353-8eae1937bf84
	github.com/openconfig/goyang v0.2.4
	github.com/openconfig/ygot v0.10.3
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	google.golang.org/grpc v1.36.1
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
	k8s.io/klog v1.0.0
)
