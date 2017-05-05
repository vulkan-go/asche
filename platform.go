package asche

import (
	"errors"
	"log"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

type Platform interface {
	// MemoryProperties gets the current Vulkan physical device memory properties.
	MemoryProperties() vk.PhysicalDeviceMemoryProperties
	// PhysicalDeviceProperies gets the current Vulkan physical device properties.
	PhysicalDeviceProperies() vk.PhysicalDeviceProperties
	// GraphicsQueueFamilyIndex gets the current Vulkan graphics queue family index.
	GraphicsQueueFamilyIndex() uint32
	// PresentQueueFamilyIndex gets the current Vulkan present queue family index.
	PresentQueueFamilyIndex() uint32
	// HasSeparatePresentQueue is true when PresentQueueFamilyIndex differs from GraphicsQueueFamilyIndex.
	HasSeparatePresentQueue() bool
	// GraphicsQueue gets the current Vulkan graphics queue.
	GraphicsQueue() vk.Queue
	// PresentQueue gets the current Vulkan present queue.
	PresentQueue() vk.Queue
	// Instance gets the current Vulkan instance.
	Instance() vk.Instance
	// Device gets the current Vulkan device.
	Device() vk.Device
	// PhysicalDevice gets the current Vulkan physical device.
	PhysicalDevice() vk.PhysicalDevice
	// Surface gets the current Vulkan surface.
	Surface() vk.Surface
	// Destroy is the destructor for the Platform instance.
	Destroy()
}

func NewPlatform(app Application) (pFace Platform, err error) {
	// defer checkErr(&err)
	p := &platform{
		basePlatform: basePlatform{
			context: &context{
				// TODO: make configurable
				// defines count of slots allocated in swapchain
				frameLag: 3,
			},
		},
	}
	p.context.platform = p

	// Select instance extensions
	requiredInstanceExtensions := safeStrings(app.VulkanInstanceExtensions())
	actualInstanceExtensions, err := InstanceExtensions()
	orPanic(err)
	instanceExtensions, missing := checkExisting(actualInstanceExtensions, requiredInstanceExtensions)
	if missing > 0 {
		log.Println("vulkan warning: missing", missing, "required instance extensions during init")
	}
	log.Printf("vulkan: enabling %d instance extensions", len(instanceExtensions))

	// Select instance layers
	var validationLayers []string
	if iface, ok := app.(ApplicationVulkanLayers); ok {
		requiredValidationLayers := safeStrings(iface.VulkanLayers())
		actualValidationLayers, err := ValidationLayers()
		orPanic(err)
		validationLayers, missing = checkExisting(actualValidationLayers, requiredValidationLayers)
		if missing > 0 {
			log.Println("vulkan warning: missing", missing, "required validation layers during init")
		}
	}

	// Create instance
	var instance vk.Instance
	ret := vk.CreateInstance(&vk.InstanceCreateInfo{
		SType: vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: &vk.ApplicationInfo{
			SType:              vk.StructureTypeApplicationInfo,
			ApiVersion:         uint32(app.VulkanAPIVersion()),
			ApplicationVersion: uint32(app.VulkanAppVersion()),
			PApplicationName:   safeString(app.VulkanAppName()),
			PEngineName:        "vulkango.com\x00",
		},
		EnabledExtensionCount:   uint32(len(instanceExtensions)),
		PpEnabledExtensionNames: instanceExtensions,
		EnabledLayerCount:       uint32(len(validationLayers)),
		PpEnabledLayerNames:     validationLayers,
	}, nil, &instance)
	orPanic(NewError(ret))
	p.instance = instance
	vk.InitInstance(instance)

	if app.VulkanDebug() {
		// Register a debug callback
		ret := vk.CreateDebugReportCallback(instance, &vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit),
			PfnCallback: dbgCallbackFunc,
		}, nil, &p.debugCallback)
		orPanic(NewError(ret))
		log.Println("vulkan: DebugReportCallback enabled by application")
	}

	// Find a suitable GPU
	var gpuCount uint32
	ret = vk.EnumeratePhysicalDevices(p.instance, &gpuCount, nil)
	orPanic(NewError(ret))
	if gpuCount == 0 {
		return nil, errors.New("vulkan error: no GPU devices found")
	}
	gpus := make([]vk.PhysicalDevice, gpuCount)
	ret = vk.EnumeratePhysicalDevices(p.instance, &gpuCount, gpus)
	orPanic(NewError(ret))
	// get the first one, multiple GPUs not supported yet
	p.gpu = gpus[0]
	vk.GetPhysicalDeviceProperties(p.gpu, &p.gpuProperties)
	p.gpuProperties.Deref()
	vk.GetPhysicalDeviceMemoryProperties(p.gpu, &p.memoryProperties)
	p.memoryProperties.Deref()

	// Select device extensions
	requiredDeviceExtensions := safeStrings(app.VulkanDeviceExtensions())
	actualDeviceExtensions, err := DeviceExtensions(p.gpu)
	orPanic(err)
	deviceExtensions, missing := checkExisting(actualDeviceExtensions, requiredDeviceExtensions)
	if missing > 0 {
		log.Println("vulkan warning: missing", missing, "required device extensions during init")
	}
	log.Printf("vulkan: enabling %d device extensions", len(deviceExtensions))

	// Make sure the surface is here if required
	mode := app.VulkanMode()
	if mode.Has(VulkanPresent) { // so, a surface is required and provided
		p.surface = app.VulkanSurface(p.instance)
		if p.surface == vk.NullSurface {
			return nil, errors.New("vulkan error: surface required but not provided")
		}
	}

	// Get queue family properties
	var queueCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(p.gpu, &queueCount, nil)
	queueProperties := make([]vk.QueueFamilyProperties, queueCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(p.gpu, &queueCount, queueProperties)
	if queueCount == 0 { // probably should try another GPU
		return nil, errors.New("vulkan error: no queue families found on GPU 0")
	}

	// Find a suitable queue family for the target Vulkan mode
	var graphicsFound bool
	var presentFound bool
	var separateQueue bool
	for i := uint32(0); i < queueCount; i++ {
		var (
			required        vk.QueueFlags
			supportsPresent vk.Bool32
			needsPresent    bool
		)
		if graphicsFound {
			// looking for separate present queue
			separateQueue = true
			vk.GetPhysicalDeviceSurfaceSupport(p.gpu, i, p.surface, &supportsPresent)
			if supportsPresent.B() {
				p.presentQueueIndex = i
				presentFound = true
				break
			}
		}
		if mode.Has(VulkanCompute) {
			required |= vk.QueueFlags(vk.QueueComputeBit)
		}
		if mode.Has(VulkanGraphics) {
			required |= vk.QueueFlags(vk.QueueGraphicsBit)
		}
		if mode.Has(VulkanPresent) {
			needsPresent = true
			vk.GetPhysicalDeviceSurfaceSupport(p.gpu, i, p.surface, &supportsPresent)
		}
		queueProperties[i].Deref()
		if queueProperties[i].QueueFlags&required != 0 {
			if !needsPresent || (needsPresent && supportsPresent.B()) {
				p.graphicsQueueIndex = i
				graphicsFound = true
				break
			} else if needsPresent {
				p.graphicsQueueIndex = i
				graphicsFound = true
				// need present, but this one doesn't support
				// continue lookup
			}
		}
	}
	if separateQueue && !presentFound {
		err := errors.New("vulkan error: could not found separate queue with present capabilities")
		return nil, err
	}
	if !graphicsFound {
		err := errors.New("vulkan error: could not find a suitable queue family for the target Vulkan mode")
		return nil, err
	}

	// Create a Vulkan device
	queueInfos := []vk.DeviceQueueCreateInfo{{
		SType:            vk.StructureTypeDeviceQueueCreateInfo,
		QueueFamilyIndex: p.graphicsQueueIndex,
		QueueCount:       1,
		PQueuePriorities: []float32{1.0},
	}}
	if separateQueue {
		queueInfos = append(queueInfos, vk.DeviceQueueCreateInfo{
			SType:            vk.StructureTypeDeviceQueueCreateInfo,
			QueueFamilyIndex: p.presentQueueIndex,
			QueueCount:       1,
			PQueuePriorities: []float32{1.0},
		})
	}

	var device vk.Device
	ret = vk.CreateDevice(p.gpu, &vk.DeviceCreateInfo{
		SType:                   vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount:    uint32(len(queueInfos)),
		PQueueCreateInfos:       queueInfos,
		EnabledExtensionCount:   uint32(len(deviceExtensions)),
		PpEnabledExtensionNames: deviceExtensions,
		EnabledLayerCount:       uint32(len(validationLayers)),
		PpEnabledLayerNames:     validationLayers,
	}, nil, &device)
	orPanic(NewError(ret))
	p.device = device
	p.context.device = device
	app.VulkanInit(p.context)

	var queue vk.Queue
	vk.GetDeviceQueue(p.device, p.graphicsQueueIndex, 0, &queue)
	p.graphicsQueue = queue

	if mode.Has(VulkanPresent) { // init a swapchain for surface
		if separateQueue {
			var presentQueue vk.Queue
			vk.GetDeviceQueue(p.device, p.presentQueueIndex, 0, &presentQueue)
			p.presentQueue = presentQueue
		}
		p.context.preparePresent()

		dimensions := &SwapchainDimensions{
			// some default preferences here
			Width: 640, Height: 480,
			Format: vk.FormatB8g8r8a8Unorm,
		}
		if iface, ok := app.(ApplicationSwapchainDimensions); ok {
			dimensions = iface.VulkanSwapchainDimensions()
		}
		p.context.prepareSwapchain(p.gpu, p.surface, dimensions)
	}
	if iface, ok := app.(ApplicationContextPrepare); ok {
		p.context.SetOnPrepare(iface.VulkanContextPrepare)
	}
	if iface, ok := app.(ApplicationContextCleanup); ok {
		p.context.SetOnCleanup(iface.VulkanContextCleanup)
	}
	if iface, ok := app.(ApplicationContextInvalidate); ok {
		p.context.SetOnInvalidate(iface.VulkanContextInvalidate)
	}
	if mode.Has(VulkanPresent) {
		p.context.prepare(false)
	}
	return p, nil
}

type basePlatform struct {
	context *context

	instance vk.Instance
	gpu      vk.PhysicalDevice
	device   vk.Device

	graphicsQueueIndex uint32
	presentQueueIndex  uint32
	presentQueue       vk.Queue
	graphicsQueue      vk.Queue

	gpuProperties    vk.PhysicalDeviceProperties
	memoryProperties vk.PhysicalDeviceMemoryProperties
}

func (p *basePlatform) MemoryProperties() vk.PhysicalDeviceMemoryProperties {
	return p.memoryProperties
}

func (p *basePlatform) PhysicalDeviceProperies() vk.PhysicalDeviceProperties {
	return p.gpuProperties
}

func (p *basePlatform) PhysicalDevice() vk.PhysicalDevice {
	return p.gpu
}

func (p *basePlatform) Surface() vk.Surface {
	return vk.NullSurface
}

func (p *basePlatform) GraphicsQueueFamilyIndex() uint32 {
	return p.graphicsQueueIndex
}

func (p *basePlatform) PresentQueueFamilyIndex() uint32 {
	return p.presentQueueIndex
}

func (p *basePlatform) HasSeparatePresentQueue() bool {
	return p.presentQueueIndex != p.graphicsQueueIndex
}

func (p *basePlatform) GraphicsQueue() vk.Queue {
	return p.graphicsQueue
}

func (p *basePlatform) PresentQueue() vk.Queue {
	if p.graphicsQueueIndex != p.presentQueueIndex {
		return p.presentQueue
	}
	return p.graphicsQueue
}

func (p *basePlatform) Instance() vk.Instance {
	return p.instance
}

func (p *basePlatform) Device() vk.Device {
	return p.device
}

type platform struct {
	basePlatform

	surface       vk.Surface
	debugCallback vk.DebugReportCallback
}

func (p *platform) Surface() vk.Surface {
	return p.surface
}

func (p *platform) Destroy() {
	if p.device != nil {
		vk.DeviceWaitIdle(p.device)
	}
	p.context.destroy()
	p.context = nil
	if p.surface != vk.NullSurface {
		vk.DestroySurface(p.instance, p.surface, nil)
		p.surface = vk.NullSurface
	}
	if p.device != nil {
		vk.DestroyDevice(p.device, nil)
		p.device = nil
	}
	if p.debugCallback != vk.NullDebugReportCallback {
		vk.DestroyDebugReportCallback(p.instance, p.debugCallback, nil)
	}
	if p.instance != nil {
		vk.DestroyInstance(p.instance, nil)
		p.instance = nil
	}
}

func dbgCallbackFunc(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType,
	object uint64, location uint, messageCode int32, pLayerPrefix string,
	pMessage string, pUserData unsafe.Pointer) vk.Bool32 {

	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportInformationBit) != 0:
		log.Printf("INFORMATION: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		log.Printf("WARNING: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportPerformanceWarningBit) != 0:
		log.Printf("PERFORMANCE WARNING: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		log.Printf("ERROR: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportDebugBit) != 0:
		log.Printf("DEBUG: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	default:
		log.Printf("INFORMATION: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	}
	return vk.Bool32(vk.False)
}
