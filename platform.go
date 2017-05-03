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
	// Queue gets the current Vulkan graphics queue.
	Queue() vk.Queue
	// Instance gets the current Vulkan instance.
	Instance() vk.Instance
	// Device gets the current Vulkan device.
	Device() vk.Device
	// PhysicalDevice gets the current Vulkan physical device.
	PhysicalDevice() vk.PhysicalDevice
	// Surface gets the current Vulkan surface.
	Surface() vk.Surface
	// CurrentSwapchain gets the current swapchain. Returns imagess which application can render into,
	// and the swapchain dimensions currently used.
	CurrentSwapchain() ([]vk.Image, *SwapchainDimensions)
	// SwapchainImagesCount gets number of swapchain images used.
	SwapchainImagesCount() uint32
	// AcquireNextImage at start of a frame acquires the next swapchain image to render into.
	// Returns the acquired image index, along with error and an outdated marker
	// that corresponds to vk.ErrorOutOfDate or vk.Suboptimal, user should update
	// their swapchain with info from CurrentSwapchain method and call AcquireNextImage again.
	AcquireNextImage() (imageIndex uint32, outdated bool, err error)
	// PresentImage presents an image to the swapchain.
	// imageIndex should be obtained from AcquireNextImage.
	PresentImage(imageIndex uint32) (outdated bool, err error)
	// Destroy is the destructor for the Platform instance.
	Destroy()
}

func NewPlatform(app Application) (pFace Platform, err error) {
	// defer checkErr(&err)
	p := &platform{
		basePlatform: basePlatform{
			context: &context{},
		},
	}

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
	p.queueProperties = make([]vk.QueueFamilyProperties, queueCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(p.gpu, &queueCount, p.queueProperties)
	if queueCount == 0 { // probably should try another GPU
		return nil, errors.New("vulkan error: no queue families found on GPU 0")
	}

	// Find a suitable queue family for the target Vulkan mode

	var foundQueue bool
	for i := uint32(0); i < queueCount; i++ {
		var (
			required        vk.QueueFlags
			supportsPresent vk.Bool32
			needsPresent    bool
		)
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
		p.queueProperties[i].Deref()
		if p.queueProperties[i].QueueFlags&required == required {
			if !needsPresent || (needsPresent && supportsPresent.B()) {
				p.graphicsQueueIndex = i
				foundQueue = true
				break
			}
		}
	}
	if !foundQueue {
		err := errors.New("vulkan error: could not find a suitable queue family for the target Vulkan mode")
		return nil, err
	}

	// Create a Vulkan device

	var device vk.Device
	ret = vk.CreateDevice(p.gpu, &vk.DeviceCreateInfo{
		SType:                vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount: 1,
		PQueueCreateInfos: []vk.DeviceQueueCreateInfo{{
			SType:            vk.StructureTypeDeviceQueueCreateInfo,
			QueueFamilyIndex: p.graphicsQueueIndex,
			QueueCount:       1,
			PQueuePriorities: []float32{
				1.0,
			},
		}},
		EnabledExtensionCount:   uint32(len(deviceExtensions)),
		PpEnabledExtensionNames: deviceExtensions,
		EnabledLayerCount:       uint32(len(validationLayers)),
		PpEnabledLayerNames:     validationLayers,
	}, nil, &device)
	orPanic(NewError(ret))
	p.device = device

	var queue vk.Queue
	vk.GetDeviceQueue(p.device, p.graphicsQueueIndex, 0, &queue)
	p.queue = queue

	if mode.Has(VulkanPresent) { // init a swapchain for surface
		dimensions := &SwapchainDimensions{
			// some default preferences here
			Width: 640, Height: 480,
			Format: vk.FormatB8g8r8a8Unorm,
		}
		if iface, ok := app.(ApplicationSwapchainDimensions); ok {
			dimensions = iface.VulkanSwapchainDimensions()
		}
		p.swapchainDimensions, err = p.context.prepareSwapchain(dimensions)
		orPanic(err)
	}

	// Finally, init the context and the application with platform state

	orPanic(p.context.OnPlatformUpdate(p))
	return p, app.VulkanInit(p.context)
}

type basePlatform struct {
	context *context

	// The vulkan instance.
	instance vk.Instance
	// The vulkan physical device.
	gpu vk.PhysicalDevice
	// The vulkan device.
	device vk.Device
	// The vulkan device queue.
	queue vk.Queue
	// The vulkan physical device properties.
	gpuProperties vk.PhysicalDeviceProperties
	// The vulkan physical device memory properties.
	memoryProperties vk.PhysicalDeviceMemoryProperties
	// The vulkan physical device queue properties.
	queueProperties []vk.QueueFamilyProperties
	// The queue family index where graphics work will be submitted.
	graphicsQueueIndex uint32
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
	return vk.NullHandle
}

func (p *basePlatform) GraphicsQueueFamilyIndex() uint32 {
	return p.graphicsQueueIndex
}

func (p *basePlatform) Queue() vk.Queue {
	return p.queue
}

func (p *basePlatform) Instance() vk.Instance {
	return p.instance
}

func (p *basePlatform) Device() vk.Device {
	return p.device
}

type platform struct {
	basePlatform

	surface             vk.Surface
	swapchain           vk.Swapchain
	swapchainDimensions *SwapchainDimensions
	swapchainImages     []vk.Image
	debugCallback       vk.DebugReportCallback
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
	if p.swapchain != vk.NullSwapchain {
		vk.DestroySwapchain(p.device, p.swapchain, nil)
		p.swapchain = vk.NullSwapchain
	}
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

func (p *platform) AcquireNextImage() (imageIndex uint32, outdated bool, err error) {
	defer checkErr(&err)

	var acquireSemaphore vk.Semaphore
	var releaseSemaphore vk.Semaphore
	semaphoreInfo := &vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	ret := vk.CreateSemaphore(p.device, semaphoreInfo, nil, &acquireSemaphore)
	orPanic(NewError(ret))
	// We will not need a semaphore since we will wait on host side for the fence to be set.
	ret = vk.AcquireNextImage(p.device, p.swapchain, vk.MaxUint64,
		acquireSemaphore, vk.NullFence, &imageIndex)
	switch ret {
	case vk.Suboptimal, vk.ErrorOutOfDate:
		vk.QueueWaitIdle(p.queue)
		vk.DestroySemaphore(p.device, acquireSemaphore, nil)
		// Recreate swapchain.
		p.swapchainDimensions, err = p.prepareSwapchain(p.swapchainDimensions)
		outdated = true
		return
	case vk.Success:
		ret = vk.CreateSemaphore(p.device, semaphoreInfo, nil, &releaseSemaphore)
		orPanic(NewError(ret), func() {
			vk.QueueWaitIdle(p.queue)
			vk.DestroySemaphore(p.device, acquireSemaphore, nil)
		})
		// Signal the underlying context that we're using this backbuffer now.
		// This will also wait for all fences associated with this swapchain image to complete first.
		p.context.BeginFrame(imageIndex, acquireSemaphore, releaseSemaphore)
		return
	default:
		vk.QueueWaitIdle(p.queue)
		vk.DestroySemaphore(p.device, acquireSemaphore, nil)
		err = NewError(ret)
		return
	}
}

func (p *platform) PresentImage(idx uint32) (outdated bool, err error) {
	ret := vk.QueuePresent(p.queue,
		&vk.PresentInfo{
			SType: vk.StructureTypePresentInfo,
			PImageIndices: []uint32{
				idx,
			},
			SwapchainCount: 1,
			PSwapchains: []vk.Swapchain{
				p.swapchain,
			},
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				p.context.getSwapchainReleaseSemaphore(),
			},
		})
	switch ret {
	case vk.Suboptimal, vk.ErrorOutOfDate:
		outdated = true
		return
	case vk.Success:
		return
	default:
		err = NewError(ret)
		return
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
		log.Printf("INFORMATION: [%s] Code %d : %s", pLayerPrefix, messageCode, pMsg)
	}
	return vk.Bool32(vk.False)
}
