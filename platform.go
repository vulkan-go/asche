package asche

import (
	"errors"
	"log"

	vk "github.com/vulkan-go/vulkan"
)

type Platform interface {
	// MemoryProperties gets the current Vulkan GPU memory properties.
	MemoryProperties() vk.PhysicalDeviceMemoryProperties
	// GPUProperies gets the current Vulkan GPU properties.
	GPUProperies() vk.PhysicalDeviceProperties
	// GraphicsQueueIndex gets the current Vulkan graphics queue family index.
	GraphicsQueueIndex() uint32
	// Queue gets the current Vulkan graphics queue.
	Queue() vk.Queue
	// Instance gets the current Vulkan instance.
	Instance() vk.Instance
	// Device gets the current Vulkan device.
	Device() vk.Device
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
	defer checkErr(&err)
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

	instanceInfo := &vk.InstanceCreateInfo{
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
	}
	ret := vk.CreateInstance(instanceInfo, nil, &p.instance)
	orPanic(newError(ret))

	// Find a suitalbe GPU

	var gpuCount uint32
	ret = vk.EnumeratePhysicalDevices(p.instance, &gpuCount, nil)
	orPanic(newError(ret))
	if gpuCount == 0 {
		return nil, errors.New("vulkan error: no GPU devices found")
	}
	gpus := make([]vk.PhysicalDevice, gpuCount)
	ret = vk.EnumeratePhysicalDevices(p.instance, &gpuCount, gpus)
	orPanic(newError(ret))
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
	mode := app.VulkanMode()
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
			vk.GetPhysicalDeviceSurfaceSupport(p.gpu, i, vk.NullSurface, &supportsPresent)
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
	}, nil, &p.device)
	orPanic(newError(ret))
	vk.GetDeviceQueue(p.device, p.graphicsQueueIndex, 0, &p.queue)

	// Make sure the surface is here if required

	if mode.Has(VulkanPresent) { // so, a surface is required and provided
		p.surface = app.VulkanSurface()
		if p.surface == vk.NullSurface {
			return nil, errors.New("vulkan error: surface required but not provided")
		}
		dimensions := SwapchainDimensions{
			// some default preferences here
			Width: 640, Height: 480,
			Format: vk.FormatB8g8r8a8Unorm,
		}
		if iface, ok := app.(ApplicationSwapchainDimensions); ok {
			dimensions = iface.VulkanSwapchainDimensions()
		}
		orPanic(p.initSwapchain(&dimensions))
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

func (p *basePlatform) GPUProperies() vk.PhysicalDeviceProperties {
	return p.gpuProperties
}

func (p *basePlatform) GraphicsQueueIndex() uint32 {
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
}

func (p *platform) Destroy() {
	// Don't release anything until the GPU is completely idle.
	if p.device != nil {
		vk.DeviceWaitIdle(p.device)
	}
	// Make sure we tear down the context before destroying the device since context
	// also owns some Vulkan resources.
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
	if p.instance != nil {
		vk.DestroyInstance(p.instance, nil)
		p.instance = nil
	}
}

func (p *platform) initSwapchain(dim *SwapchainDimensions) error {

	// Read surface capabilities

	var surfaceCapabilities vk.SurfaceCapabilities
	ret := vk.GetPhysicalDeviceSurfaceCapabilities(p.gpu, p.surface, &surfaceCapabilities)
	orPanic(newError(ret))
	surfaceCapabilities.Deref()

	// Get available surface pixel formats

	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(p.gpu, p.surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(p.gpu, p.surface, &formatCount, formats)

	// Select a proper surface format

	var format vk.SurfaceFormat
	if formatCount == 1 {
		formats[0].Deref()
		if formats[0].Format == vk.FormatUndefined {
			format = formats[0]
			format.Format = dim.Format
		} else {
			format = formats[0]
		}
	} else if formatCount == 0 {
		return errors.New("vulkan error: surface has no pixel formats")
	} else {
		formats[0].Deref()
		// select the first one available
		format = formats[0]
	}

	// Setup swapchain parameters

	var swapchainSize vk.Extent2D
	surfaceCapabilities.CurrentExtent.Deref()
	if surfaceCapabilities.CurrentExtent.Width == vk.MaxUint32 {
		swapchainSize.Width = dim.Width
		swapchainSize.Height = dim.Height
	} else {
		swapchainSize = surfaceCapabilities.CurrentExtent
	}
	// FIFO must be supported by all implementations.
	swapchainPresentMode := vk.PresentModeFifo
	// Determine the number of VkImage's to use in the swapchain.
	// Ideally, we desire to own 1 image at a time, the rest of the images can either be rendered to and/or
	// being queued up for display.
	desiredSwapchainImages := surfaceCapabilities.MinImageCount + 1
	if surfaceCapabilities.MaxImageCount > 0 && desiredSwapchainImages > surfaceCapabilities.MaxImageCount {
		// Application must settle for fewer images than desired.
		desiredSwapchainImages = surfaceCapabilities.MaxImageCount
	}

	// Figure out a suitable surface transform.

	var preTransform vk.SurfaceTransformFlagBits
	requiredTransforms := vk.SurfaceTransformIdentityBit
	supportedTransforms := vk.SurfaceTransformFlagBits(surfaceCapabilities.SupportedTransforms)
	if supportedTransforms&requiredTransforms == requiredTransforms {
		preTransform = requiredTransforms
	} else {
		preTransform = surfaceCapabilities.CurrentTransform
	}

	// Create a swapchain

	oldSwapchain := p.swapchain
	ret = vk.CreateSwapchain(p.device, &vk.SwapchainCreateInfo{
		Surface:         p.surface,
		MinImageCount:   desiredSwapchainImages,
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  swapchainSize.Width,
			Height: swapchainSize.Height,
		},
		ImageArrayLayers: 1,
		ImageUsage:       vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		ImageSharingMode: vk.SharingModeExclusive,
		PreTransform:     preTransform,
		CompositeAlpha:   vk.CompositeAlphaOpaqueBit,
		PresentMode:      swapchainPresentMode,
		Clipped:          vk.True,
		OldSwapchain:     oldSwapchain,
	}, nil, &p.swapchain)
	orPanic(newError(ret))
	if oldSwapchain != vk.NullSwapchain {
		vk.DestroySwapchain(p.device, oldSwapchain, nil)
	}

	// Save properties and get swapchain images

	p.swapchainDimensions.Width = swapchainSize.Width
	p.swapchainDimensions.Height = swapchainSize.Height
	p.swapchainDimensions.Format = format.Format

	var imageCount uint32
	ret = vk.GetSwapchainImages(p.device, p.swapchain, &imageCount, nil)
	orPanic(newError(ret))
	p.swapchainImages = make([]vk.Image, imageCount)
	ret = vk.GetSwapchainImages(p.device, p.swapchain, &imageCount, p.swapchainImages)
	orPanic(newError(ret))

	return nil
}

func (p *platform) CurrentSwapchain() ([]vk.Image, *SwapchainDimensions) {
	return p.swapchainImages, p.swapchainDimensions
}

func (p *platform) SwapchainImagesCount() uint32 {
	return uint32(len(p.swapchainImages))
}

func (p *platform) AcquireNextImage() (imageIndex uint32, outdated bool, err error) {
	defer checkErr(&err)

	var acquireSemaphore vk.Semaphore
	var releaseSemaphore vk.Semaphore
	semaphoreInfo := &vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	ret := vk.CreateSemaphore(p.device, semaphoreInfo, nil, &acquireSemaphore)
	orPanic(newError(ret))
	// We will not need a semaphore since we will wait on host side for the fence to be set.
	ret = vk.AcquireNextImage(p.device, p.swapchain, vk.MaxUint64,
		acquireSemaphore, vk.NullFence, &imageIndex)
	switch ret {
	case vk.Suboptimal, vk.ErrorOutOfDate:
		vk.QueueWaitIdle(p.queue)
		vk.DestroySemaphore(p.device, acquireSemaphore, nil)
		// Recreate swapchain.
		err = p.initSwapchain(p.swapchainDimensions)
		outdated = true
		return
	case vk.Success:
		ret = vk.CreateSemaphore(p.device, semaphoreInfo, nil, &releaseSemaphore)
		orPanic(newError(ret), func() {
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
		err = newError(ret)
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
		err = newError(ret)
		return
	}
}
