package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog"

	// "srlinux.io/kubemgr/pkg/storage"
	"srlinux.io/kubemgr/internal/agent"
	"srlinux.io/kubemgr/internal/cpumgr"
	"srlinux.io/kubemgr/internal/k8s"
	"srlinux.io/kubemgr/internal/netmgr"

	"github.com/vishvananda/netns"
)

const (
	ndkAddress    = "localhost:50053"
	agentName     = "kube_mgr"
	yangRoot      = ".kubernetes"
	reservedCores = 1
)

// Global vars
var (
	KubeMgr agent.Agent
)

// SetName publishes the baremetal's hostname into the container
func SetName(nodeName string) {
	log.Infof("Setting node name...")
	// KubeMgr.Yang.NodeName = &nodeName
	KubeMgr.Yang.SetNodeName(nodeName)
	//KubeMgr.Yang.NodeName.Value = nodeName
	// JsData, err := json.Marshal(KubeMgr.Yang)
	// if err != nil {
	// 	log.Fatalf("Can not marshal config data: error %s", err)
	// }
	// JsString := string(JsData)
	// log.Infof("JsPath: %s", KubeMgr.YangRoot)
	// log.Infof("JsString: %s", JsString)
	KubeMgr.UpdateTelemetry()
	// ndk.UpdateTelemetry(KubeMgr, &JsPath, &JsString)
}

// PodCounterMgr counts pods!
func PodCounterMgr(clientSet *kubernetes.Clientset, nodeName string) {
	for {
		totalPodsCluster, totalPodsLocal := k8s.PodCounter(clientSet, nodeName)
		KubeMgr.Yang.SetPods(totalPodsCluster, totalPodsLocal)
		KubeMgr.UpdateTelemetry()
		time.Sleep(5 * time.Second)
	}
}

func main() {
	var err error
	nodeName := os.Getenv("KUBERNETES_NODE_NAME")
	nodeIP := os.Getenv("KUBERNETES_NODE_IP")

	srlNS, err := netns.Get()
	log.Infof("Running in namespace: %#v", srlNS)
	toWrite := []byte(fmt.Sprint(srlNS))
	err = ioutil.WriteFile("/etc/opt/srlinux/namespace", toWrite, 0644)
	if err != nil {
		log.Fatalf("Unable to write namespace: %v to file /etc/opt/srlinux/namespace")
	}

	log.Infof("Initializing NDK...")
	KubeMgr = agent.Agent{}
	KubeMgr.Init(agentName, ndkAddress, yangRoot)

	log.Infof("Starting to receive notifications from NDK...")
	KubeMgr.Wg.Add(1)
	go KubeMgr.ReceiveNotifications()

	time.Sleep(10 * time.Second)
	log.Infof("Initializing K8 client...")
	KubeClientSet := k8s.K8Init()

	log.Infof("Starting PodCounterMgr...")
	KubeMgr.Wg.Add(1)
	go PodCounterMgr(KubeClientSet, nodeName)

	log.Infof("Starting CPUMgr...")
	KubeMgr.Wg.Add(1)
	go cpumgr.CPUMgr(KubeClientSet, &KubeMgr)

	time.Sleep(10 * time.Second)
	log.Infof("Starting NetMgr...")
	KubeMgr.Wg.Add(1)
	go netmgr.NetMgr(KubeClientSet, nodeIP, &KubeMgr)

	time.Sleep(10 * time.Second)
	log.Infof("Starting NameMgr...")
	KubeMgr.Wg.Add(1)
	go SetName(nodeName)

	KubeMgr.Wg.Wait()

	KubeMgr.GrpcConn.Close()
}
