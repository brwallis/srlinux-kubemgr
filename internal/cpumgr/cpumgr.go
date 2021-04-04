package cpumgr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"srlinux.io/kubemgr/internal/agent"
	"srlinux.io/kubemgr/internal/cpu"

	log "k8s.io/klog"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"srlinux.io/go/pkg/gnmi"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	yangRoot      = ".kubernetes"
	reservedCores = 1
)

var (
	KubeMgr *agent.Agent
)

type YangCPUSet struct {
	CPUSet []int `json:"cpu-set"`
}

// ConfigMapController struct
type ConfigMapController struct {
	informerFactory   informers.SharedInformerFactory
	configMapInformer coreinformers.ConfigMapInformer
}

// processVxdpCPUConfigMap extracts vXDP CPU configuration from a ConfigMap, and processes it
// processing involves updating telemetry, figuring out a valid CPU set to use based on the system topology
// and then setting the derived CPU set to vSRL via gNMI
func processVxdpCPUConfigMap(configMap *v1.ConfigMap) {
	log.Infof("ConfigMap has max/min CPUs: %s/%s", configMap.Data["vxdp_cpu_max"], configMap.Data["vxdp_cpu_min"])
	vXDPCPUMax, err := strconv.ParseUint(configMap.Data["vxdp_cpu_max"], 10, 32)
	vXDPCPUMin, err := strconv.ParseUint(configMap.Data["vxdp_cpu_min"], 10, 32)
	// *KubeMgr.Yang.CpuMax = uint32(vXDPCPUMax)
	// *KubeMgr.Yang.CpuMin = uint32(vXDPCPUMin)
	KubeMgr.Yang.SetCPU(uint32(vXDPCPUMin), uint32(vXDPCPUMax))
	// KubeMgr.Yang.CPUMax.Value = uint32(vXDPCPUMax)
	// KubeMgr.Yang.CPUMin.Value = uint32(vXDPCPUMin)
	// JsData, err := json.Marshal(KubeMgr.Yang)
	// if err != nil {
	// 	log.Fatalf("Can not marshal config data: error %s", err)
	// }
	// JsString := string(JsData)
	// log.Infof("JsPath: %s", KubeMgr.YangRoot)
	// log.Infof("JsString: %s", JsString)

	KubeMgr.UpdateTelemetry()
	// ndk.UpdateTelemetry(KubeMgr, &JsPath, &JsString)
	// Figure out the CPU to use based on the min, first resolve the current list of CPUs to a map of cores
	coreList := cpu.FindCoreThreadPairs(cpu.EntryCPUSet())
	cpuAll := cpu.TotalCPUs()
	reservedCPUs := reservedCores * 2
	var vXDPCoreList []cpu.Core
	var reservedCoreList []cpu.Core
	// Divide the vxdp_cpu_min by 2
	if vXDPCPUMin%2 == 0 {
		// We can safely divide
		vXDPCoreMin := int(vXDPCPUMin) / 2
		// Ensure we have enough resources based on what is available
		if int(vXDPCPUMin) > (cpuAll - reservedCPUs) {
			fmt.Printf("Not enough CPUs available to serve min CPU ask. Available: %d, requested: %d", cpuAll, vXDPCoreMin)
		} else {
			// Reserve cores first
			for i := 0; i < reservedCores; i++ {
				// Reserve a core
				reservedCoreList = append(reservedCoreList, coreList[i])
			}
			// vXDP cores next
			for i := 0; i < vXDPCoreMin; i++ {
				// Reserve a core
				vXDPCoreList = append(vXDPCoreList, coreList[i+reservedCores])
			}
			log.Infof("Reserved cores: %v", reservedCoreList)
			log.Infof("vXDP cores: %v", vXDPCoreList)
		}
	}
	var vXDPCPUList []int
	// Build the vXDP CPU list
	for _, core := range vXDPCoreList {
		cpuList := core.GetChildren()
		for _, cpu := range cpuList {
			vXDPCPUList = append(vXDPCPUList, cpu)
		}
	}
	log.Infof("vXDP CPUs: %v", vXDPCPUList)

	// Build JSON
	JSONCPUSet := &YangCPUSet{
		CPUSet: vXDPCPUList,
	}
	out, err := json.MarshalIndent(JSONCPUSet, "", "  ")
	if err != nil {
		log.Infof("Unable to Marshal CPU set: %e", err)
	}
	gNMICPUSet := &gpb.TypedValue{
		Value: &gpb.TypedValue_JsonIetfVal{
			JsonIetfVal: out,
		},
	}
	gnmi.Set("/platform/vxdp", gNMICPUSet)
}

// Run starts shared informers and waits for the shared informer cache to synchronize
func (c *ConfigMapController) Run(stopCh chan struct{}) error {
	// Starts all the shared informers that have been created by the factory so far
	c.informerFactory.Start(stopCh)
	// wait for the initial synchronization of the local cache
	if !cache.WaitForCacheSync(stopCh, c.configMapInformer.Informer().HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

func (c *ConfigMapController) configMapAdd(obj interface{}) {
	configMap := obj.(*v1.ConfigMap)
	log.Infof("ConfigMap CREATED: %s/%s", configMap.Namespace, configMap.Name)
	if configMap.Namespace == "kube-system" {
		if configMap.Name == "srlinux-config" {
			log.Infof("ConfigMap has the correct name: %s", configMap.Name)
			processVxdpCPUConfigMap(configMap)
		}
	}
}

func (c *ConfigMapController) configMapUpdate(old, new interface{}) {
	oldConfigMap := old.(*v1.ConfigMap)
	newConfigMap := new.(*v1.ConfigMap)
	log.Infof(
		"ConfigMap UPDATED. %s/%s %s",
		oldConfigMap.Namespace, oldConfigMap.Name, newConfigMap.Name,
	)
	if newConfigMap.Namespace == "kube-system" {
		if newConfigMap.Name == "srlinux-config" {
			log.Infof("ConfigMap has the correct name: %s", newConfigMap.Name)
			processVxdpCPUConfigMap(newConfigMap)
		}
	}
}

func (c *ConfigMapController) configMapDelete(obj interface{}) {
	configMap := obj.(*v1.ConfigMap)
	log.Infof("ConfigMap DELETED: %s/%s", configMap.Namespace, configMap.Name)
}

// NewConfigMapController creates a ConfigMapController
func NewConfigMapController(informerFactory informers.SharedInformerFactory) *ConfigMapController {
	configMapInformer := informerFactory.Core().V1().ConfigMaps()

	c := &ConfigMapController{
		informerFactory:   informerFactory,
		configMapInformer: configMapInformer,
	}
	configMapInformer.Informer().AddEventHandler(
		// Your custom resource event handlers.
		cache.ResourceEventHandlerFuncs{
			// Called on creation
			AddFunc: c.configMapAdd,
			// Called on resource update and every resyncPeriod on existing resources.
			UpdateFunc: c.configMapUpdate,
			// Called on resource deletion.
			DeleteFunc: c.configMapDelete,
		},
	)
	return c
}

// CPUMgr manages updates of ConfigMaps from K8
func CPUMgr(clientSet *kubernetes.Clientset, kubeMgr *agent.Agent) {
	KubeMgr = kubeMgr
	informerFactory := informers.NewSharedInformerFactory(clientSet, time.Hour*24)
	controller := NewConfigMapController(informerFactory)

	stop := make(chan struct{})
	defer close(stop)
	err := controller.Run(stop)
	if err != nil {
		log.Fatal(err)
	}
	select {}
}
