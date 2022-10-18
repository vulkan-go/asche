package dieselvk

import (
	"log"
	"os"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/vulkan-go/vulkan"
)

//Base DieselVK Core vulkan manager with GLFW native host management
//Core structure properties are private members to enforce future interface
//compliance with outside packages. The Vulkan core manager manages the availability
//of devices and capabilities to enfore instance creation and management. Also holds
//global type information which could be useful to multiple vulkan instances in an application
//which includes buffers and textures. Light objects in Vulkan do not always warrant a Core Abstraction
type BaseCore struct {

	//Core Implementation Context Properties
	display    CoreDisplay
	core_props map[string]*Usage
	name       string
	info_log   *log.Logger
	error_log  *log.Logger
	warn_log   *log.Logger

	//Map string id & tagging
	instance_names []string

	//List of device bidings
	logical_devices map[string]CoreDevice

	//Per Instance/Device Handles where key is the instance global id key used for accessing other held resources
	instances map[string]*CoreRenderInstance //Key: (Instance_Name) Value: Vulkan Instance

	//Images/Buffer Data
	images         map[string]CoreImage  //Key: (Unique Image ID)
	vertex_buffers map[string]CoreBuffer //Key: Unique Buffer Key
	indice_buffers map[string]CoreBuffer //Key: Unique Buffer Key
	uv_buffers     map[string]CoreBuffer //Key: Unique Buffer Key
	color_buffers  map[string]CoreBuffer //Key: Unique Buffer Key
	attr_buffers   map[string]CoreBuffer //Key: Unique Buffer Key

	//Shaders
	shaders  *CoreShader
	uniforms map[string]int //Uniform location mapping

}

//Instanitates a new core context allocation sizes, default allocation prevents buffer copies but is just used to instantiate map members
func NewBaseCore(usages map[string]*Usage, instances []string, app_name string, map_allocate_size int, buffer_instance_allocate_size int, window *glfw.Window) *BaseCore {
	var core BaseCore

	info_file, err := os.OpenFile("info_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	error_file, err := os.OpenFile("error_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	warn_file, err := os.OpenFile("warn_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	core.core_props = usages
	core.info_log = log.New(info_file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	core.error_log = log.New(error_file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	core.warn_log = log.New(warn_file, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)

	core.instance_names = instances
	core.name = app_name

	core.logical_devices = make(map[string]CoreDevice, map_allocate_size)
	core.instances = make(map[string]*CoreRenderInstance, map_allocate_size)

	core.images = make(map[string]CoreImage, buffer_instance_allocate_size)
	core.vertex_buffers = make(map[string]CoreBuffer, buffer_instance_allocate_size)
	core.indice_buffers = make(map[string]CoreBuffer, buffer_instance_allocate_size)
	core.uv_buffers = make(map[string]CoreBuffer, buffer_instance_allocate_size)
	core.color_buffers = make(map[string]CoreBuffer, buffer_instance_allocate_size)
	core.attr_buffers = make(map[string]CoreBuffer, buffer_instance_allocate_size)
	core.uniforms = make(map[string]int, buffer_instance_allocate_size)

	if window != nil && usages["Config"].String_props["Display"] == "Window" {
		core.display = CoreDisplay{
			window: window,
		}
	}

	core.CreateGraphicsInstance("Render")

	return &core
}

func (base *BaseCore) CreateGraphicsInstance(instance_name string) {

	layers := base.GetValidationLayers()
	devices := base.GetDeviceExtensions()
	instance_extensions := base.GetInstanceExtensions()
	required := base.display.window.GetRequiredInstanceExtensions()

	inst_ext := NewBaseInstanceExtensions(instance_extensions, required)
	layer_ext := NewBaseLayerExtensions(layers)

	//Create instance
	var instance vk.Instance
	var flags vk.InstanceCreateFlags
	if PlatformOS == "Darwin" {
		flags = vk.InstanceCreateFlags(0x00000001) //VK_INSTANCE_CREATE_ENUMERATE_PORTABILITY_BIT
	} else {
		flags = vk.InstanceCreateFlags(0)
	}

	//Vulkan Create Info Binding
	ret := vk.CreateInstance(&vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			ApiVersion:         uint32(vk.MakeVersion(1, 1, 0)),
			ApplicationVersion: uint32(vk.MakeVersion(1, 1, 0)),
			PApplicationName:   safeString(instance_name),
			PEngineName:        base.name + "\x00",
		},
		EnabledExtensionCount:   uint32(len(inst_ext.GetExtensions())),
		PpEnabledExtensionNames: inst_ext.GetExtensions(),
		EnabledLayerCount:       uint32(len(layer_ext.GetExtensions())),
		PpEnabledLayerNames:     layer_ext.GetExtensions(),
		Flags:                   flags,
	}, nil, &instance)

	if ret != vk.Success {
		base.error_log.Fatalf("Error creating instance with required extensions\n")
	}

	if PlatformOS == "Darwin" {
		vk.InitInstance(instance)
	}

	var err error
	var shader_map map[string]int
	shader_map = make(map[string]int, 2)
	dirs, derr := os.Getwd()
	if derr != nil {
		Fatal(derr)
	}

	shader_map[dirs+"/shaders/vert.spv"] = VERTEX
	shader_map[dirs+"/shaders/frag.spv"] = FRAG
	base.instances[instance_name], err = NewCoreRenderInstance(instance, "CoreRender", *layer_ext, devices, &base.display, NewCoreShader(shader_map, 1))

	if err != nil {
		base.error_log.Print(err)
	}

}

func (base *BaseCore) GetInstance(name string) *CoreRenderInstance {
	return base.instances[name]
}

func (base *BaseCore) GetValidationLayers() []string {
	return []string{
		//	"VK_LAYER_KHRONOS_profiles",
		"VK_LAYER_KHRONOS_synchronization2",
		"VK_LAYER_KHRONOS_validation",
		//"VK_LAYER_LUNARG_api_dump",
	}
}
func (base *BaseCore) GetDeviceExtensions() []string {
	return []string{"VK_KHR_swapchain", "VK_KHR_external_fence", "VK_KHR_portability_subset",
		"VK_KHR_external_semaphore", "VK_KHR_metal_objects", "VK_KHR_device_group"}
}

func (base *BaseCore) GetInstanceExtensions() []string {
	return []string{}
}
