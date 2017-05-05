package asche

import (
	"errors"

	vk "github.com/vulkan-go/vulkan"
)

type Context interface {
	// SetOnPrepare sets callback that will be invoked to initialize and prepare application's vulkan state
	// upon context prepare step. onCreate could create textures and pipelines,
	// descriptor layouts and render passes.
	SetOnPrepare(onPrepare func() error)
	// SetOnCleanup sets callback that will be invoked to cleanup application's vulkan state
	// upon context prepare step. onCreate could destroy textures and pipelines,
	// descriptor layouts and render passes.
	SetOnCleanup(onCleanup func() error)
	// SetOnInvalidate sets callback that will be invoked when context has been invalidated,
	// the application must update its state and prepare the corresponding swapchain image to be presented.
	// onInvalidate could compute new vertex and color data in swapchain image resource buffers.
	SetOnInvalidate(onInvalidate func(imageIdx int) error)
	// Device gets the Vulkan device assigned to the context.
	Device() vk.Device
	// Platform gets the current platform.
	Platform() Platform
	// CommandBuffer gets a command buffer currently active.
	CommandBuffer() vk.CommandBuffer
	// SwapchainDimensions gets the current swapchain dimensions, including pixel format.
	SwapchainDimensions() *SwapchainDimensions
	// SwapchainImageResources exposes the swapchain initialized image resources.
	SwapchainImageResources() []*SwapchainImageResources
	// AcquireNextImage
	AcquireNextImage() (imageIndex int, outdated bool, err error)
	// PresentImage
	PresentImage(imageIdx int) (outdated bool, err error)
}

type context struct {
	platform Platform
	device   vk.Device

	onPrepare    func() error
	onCleanup    func() error
	onInvalidate func(imageIdx int) error

	cmd            vk.CommandBuffer
	cmdPool        vk.CommandPool
	presentCmdPool vk.CommandPool

	swapchain               vk.Swapchain
	swapchainDimensions     *SwapchainDimensions
	swapchainImageResources []*SwapchainImageResources
	frameLag                int

	imageAcquiredSemaphores  []vk.Semaphore
	drawCompleteSemaphores   []vk.Semaphore
	imageOwnershipSemaphores []vk.Semaphore

	frameIndex int
}

func (c *context) preparePresent() {
	// Create semaphores to synchronize acquiring presentable buffers before
	// rendering and waiting for drawing to be complete before presenting
	semaphoreCreateInfo := &vk.SemaphoreCreateInfo{
		SType: vk.StructureTypeSemaphoreCreateInfo,
	}
	c.imageAcquiredSemaphores = make([]vk.Semaphore, c.frameLag)
	c.drawCompleteSemaphores = make([]vk.Semaphore, c.frameLag)
	c.imageOwnershipSemaphores = make([]vk.Semaphore, c.frameLag)
	for i := 0; i < c.frameLag; i++ {
		ret := vk.CreateSemaphore(c.device, semaphoreCreateInfo, nil, &c.imageAcquiredSemaphores[i])
		orPanic(NewError(ret))
		ret = vk.CreateSemaphore(c.device, semaphoreCreateInfo, nil, &c.drawCompleteSemaphores[i])
		orPanic(NewError(ret))
		if c.platform.HasSeparatePresentQueue() {
			ret = vk.CreateSemaphore(c.device, semaphoreCreateInfo, nil, &c.imageOwnershipSemaphores[i])
			orPanic(NewError(ret))
		}
	}
}

func (c *context) destroy() {
	func() (err error) {
		checkErr(&err)
		if c.onCleanup != nil {
			err = c.onCleanup()
		}
		return
	}()

	for i := 0; i < c.frameLag; i++ {
		vk.DestroySemaphore(c.device, c.imageAcquiredSemaphores[i], nil)
		vk.DestroySemaphore(c.device, c.drawCompleteSemaphores[i], nil)
		if c.platform.HasSeparatePresentQueue() {
			vk.DestroySemaphore(c.device, c.imageOwnershipSemaphores[i], nil)
		}
	}
	for i := 0; i < len(c.swapchainImageResources); i++ {
		c.swapchainImageResources[i].Destroy(c.device, c.cmdPool)
	}
	c.swapchainImageResources = nil
	if c.swapchain != vk.NullSwapchain {
		vk.DestroySwapchain(c.device, c.swapchain, nil)
		c.swapchain = vk.NullSwapchain
	}
	vk.DestroyCommandPool(c.device, c.cmdPool, nil)
	if c.platform.HasSeparatePresentQueue() {
		vk.DestroyCommandPool(c.device, c.presentCmdPool, nil)
	}
	c.platform = nil
}

func (c *context) Device() vk.Device {
	return c.device
}

func (c *context) Platform() Platform {
	return c.platform
}

func (c *context) CommandBuffer() vk.CommandBuffer {
	return c.cmd
}

func (c *context) SwapchainDimensions() *SwapchainDimensions {
	return c.swapchainDimensions
}

func (c *context) SwapchainImageResources() []*SwapchainImageResources {
	return c.swapchainImageResources
}

func (c *context) SetOnPrepare(onPrepare func() error) {
	c.onPrepare = onPrepare
}

func (c *context) SetOnCleanup(onCleanup func() error) {
	c.onCleanup = onCleanup
}

func (c *context) SetOnInvalidate(onInvalidate func(imageIdx int) error) {
	c.onInvalidate = onInvalidate
}

func (c *context) prepare(needCleanup bool) {
	vk.DeviceWaitIdle(c.device)

	if needCleanup {
		if c.onCleanup != nil {
			orPanic(c.onCleanup())
		}

		vk.DestroyCommandPool(c.device, c.cmdPool, nil)
		if c.platform.HasSeparatePresentQueue() {
			vk.DestroyCommandPool(c.device, c.presentCmdPool, nil)
		}
	}

	var cmdPool vk.CommandPool
	ret := vk.CreateCommandPool(c.device, &vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: c.platform.GraphicsQueueFamilyIndex(),
	}, nil, &cmdPool)
	orPanic(NewError(ret))
	c.cmdPool = cmdPool

	var cmd = make([]vk.CommandBuffer, 1)
	ret = vk.AllocateCommandBuffers(c.device, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        c.cmdPool,
		Level:              vk.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}, cmd)
	orPanic(NewError(ret))
	c.cmd = cmd[0]

	ret = vk.BeginCommandBuffer(c.cmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
	})
	orPanic(NewError(ret))

	for i := 0; i < len(c.swapchainImageResources); i++ {
		var cmd = make([]vk.CommandBuffer, 1)
		vk.AllocateCommandBuffers(c.device, &vk.CommandBufferAllocateInfo{
			SType:              vk.StructureTypeCommandBufferAllocateInfo,
			CommandPool:        c.cmdPool,
			Level:              vk.CommandBufferLevelPrimary,
			CommandBufferCount: 1,
		}, cmd)
		orPanic(NewError(ret))
		c.swapchainImageResources[i].cmd = cmd[0]
	}

	if c.platform.HasSeparatePresentQueue() {
		var cmdPool vk.CommandPool
		ret = vk.CreateCommandPool(c.device, &vk.CommandPoolCreateInfo{
			SType:            vk.StructureTypeCommandPoolCreateInfo,
			QueueFamilyIndex: c.platform.PresentQueueFamilyIndex(),
		}, nil, &cmdPool)
		orPanic(NewError(ret))
		c.presentCmdPool = cmdPool

		for i := 0; i < len(c.swapchainImageResources); i++ {
			var cmd = make([]vk.CommandBuffer, 1)
			ret = vk.AllocateCommandBuffers(c.device, &vk.CommandBufferAllocateInfo{
				SType:              vk.StructureTypeCommandBufferAllocateInfo,
				CommandPool:        c.presentCmdPool,
				Level:              vk.CommandBufferLevelPrimary,
				CommandBufferCount: 1,
			}, cmd)
			orPanic(NewError(ret))
			c.swapchainImageResources[i].graphicsToPresentCmd = cmd[0]

			c.swapchainImageResources[i].SetImageOwnership(
				c.platform.GraphicsQueueFamilyIndex(), c.platform.PresentQueueFamilyIndex())
		}
	}

	for i := 0; i < len(c.swapchainImageResources); i++ {
		var view vk.ImageView
		ret = vk.CreateImageView(c.device, &vk.ImageViewCreateInfo{
			SType:  vk.StructureTypeImageViewCreateInfo,
			Format: c.swapchainDimensions.Format,
			Components: vk.ComponentMapping{
				R: vk.ComponentSwizzleR,
				G: vk.ComponentSwizzleG,
				B: vk.ComponentSwizzleB,
				A: vk.ComponentSwizzleA,
			},
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			},
			ViewType: vk.ImageViewType2d,
			Image:    c.swapchainImageResources[i].image,
		}, nil, &view)
		orPanic(NewError(ret))
		c.swapchainImageResources[i].view = view
	}

	if c.onPrepare != nil {
		orPanic(c.onPrepare())
	}
	c.flushInitCmd()
}

func (c *context) flushInitCmd() {
	if c.cmd == nil {
		return
	}
	ret := vk.EndCommandBuffer(c.cmd)
	orPanic(NewError(ret))

	var fence vk.Fence
	ret = vk.CreateFence(c.device, &vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
	}, nil, &fence)
	orPanic(NewError(ret))

	cmdBufs := []vk.CommandBuffer{c.cmd}
	ret = vk.QueueSubmit(c.platform.GraphicsQueue(), 1, []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    cmdBufs,
	}}, fence)
	orPanic(NewError(ret))

	ret = vk.WaitForFences(c.device, 1, []vk.Fence{fence}, vk.True, vk.MaxUint64)
	orPanic(NewError(ret))

	vk.FreeCommandBuffers(c.device, c.cmdPool, 1, cmdBufs)
	vk.DestroyFence(c.device, fence, nil)
	c.cmd = nil
}

func (c *context) prepareSwapchain(gpu vk.PhysicalDevice, surface vk.Surface, dimensions *SwapchainDimensions) {
	// Read surface capabilities
	var surfaceCapabilities vk.SurfaceCapabilities
	ret := vk.GetPhysicalDeviceSurfaceCapabilities(gpu, surface, &surfaceCapabilities)
	orPanic(NewError(ret))
	surfaceCapabilities.Deref()

	// Get available surface pixel formats
	var formatCount uint32
	vk.GetPhysicalDeviceSurfaceFormats(gpu, surface, &formatCount, nil)
	formats := make([]vk.SurfaceFormat, formatCount)
	vk.GetPhysicalDeviceSurfaceFormats(gpu, surface, &formatCount, formats)

	// Select a proper surface format
	var format vk.SurfaceFormat
	if formatCount == 1 {
		formats[0].Deref()
		if formats[0].Format == vk.FormatUndefined {
			format = formats[0]
			format.Format = dimensions.Format
		} else {
			format = formats[0]
		}
	} else if formatCount == 0 {
		orPanic(errors.New("vulkan error: surface has no pixel formats"))
	} else {
		formats[0].Deref()
		// select the first one available
		format = formats[0]
	}

	// Setup swapchain parameters
	var swapchainSize vk.Extent2D
	surfaceCapabilities.CurrentExtent.Deref()
	if surfaceCapabilities.CurrentExtent.Width == vk.MaxUint32 {
		swapchainSize.Width = dimensions.Width
		swapchainSize.Height = dimensions.Height
	} else {
		swapchainSize = surfaceCapabilities.CurrentExtent
	}
	// The FIFO present mode is guaranteed by the spec to be supported
	// and to have no tearing.  It's a great default present mode to use.
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
	supportedTransforms := surfaceCapabilities.SupportedTransforms
	if vk.SurfaceTransformFlagBits(supportedTransforms)&requiredTransforms != 0 {
		preTransform = requiredTransforms
	} else {
		preTransform = surfaceCapabilities.CurrentTransform
	}

	// Find a supported composite alpha mode - one of these is guaranteed to be set
	compositeAlpha := vk.CompositeAlphaOpaqueBit
	compositeAlphaFlags := []vk.CompositeAlphaFlagBits{
		vk.CompositeAlphaOpaqueBit,
		vk.CompositeAlphaPreMultipliedBit,
		vk.CompositeAlphaPostMultipliedBit,
		vk.CompositeAlphaInheritBit,
	}
	for i := 0; i < len(compositeAlphaFlags); i++ {
		alphaFlags := vk.CompositeAlphaFlags(compositeAlphaFlags[i])
		flagSupported := surfaceCapabilities.SupportedCompositeAlpha&alphaFlags != 0
		if flagSupported {
			compositeAlpha = compositeAlphaFlags[i]
			break
		}
	}

	// Create a swapchain
	var swapchain vk.Swapchain
	oldSwapchain := c.swapchain
	ret = vk.CreateSwapchain(c.device, &vk.SwapchainCreateInfo{
		SType:           vk.StructureTypeSwapchainCreateInfo,
		Surface:         surface,
		MinImageCount:   desiredSwapchainImages, // 1 - 3?
		ImageFormat:     format.Format,
		ImageColorSpace: format.ColorSpace,
		ImageExtent: vk.Extent2D{
			Width:  swapchainSize.Width,
			Height: swapchainSize.Height,
		},
		ImageUsage:       vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		PreTransform:     preTransform,
		CompositeAlpha:   compositeAlpha,
		ImageArrayLayers: 1,
		ImageSharingMode: vk.SharingModeExclusive,
		PresentMode:      swapchainPresentMode,
		OldSwapchain:     oldSwapchain,
		Clipped:          vk.True,
	}, nil, &swapchain)
	orPanic(NewError(ret))
	if oldSwapchain != vk.NullSwapchain {
		vk.DestroySwapchain(c.device, oldSwapchain, nil)
	}
	c.swapchain = swapchain

	c.swapchainDimensions = &SwapchainDimensions{
		Width:  swapchainSize.Width,
		Height: swapchainSize.Height,
		Format: format.Format,
	}

	var imageCount uint32
	ret = vk.GetSwapchainImages(c.device, c.swapchain, &imageCount, nil)
	orPanic(NewError(ret))
	swapchainImages := make([]vk.Image, imageCount)
	ret = vk.GetSwapchainImages(c.device, c.swapchain, &imageCount, swapchainImages)
	orPanic(NewError(ret))
	for i := 0; i < len(c.swapchainImageResources); i++ {
		c.swapchainImageResources[i].Destroy(c.device, c.cmdPool)
	}
	c.swapchainImageResources = make([]*SwapchainImageResources, 0, imageCount)
	for i := 0; i < len(swapchainImages); i++ {
		c.swapchainImageResources = append(c.swapchainImageResources, &SwapchainImageResources{
			image: swapchainImages[i],
		})
	}
}

func (c *context) AcquireNextImage() (imageIndex int, outdated bool, err error) {
	defer checkErr(&err)

	// Get the index of the next available swapchain image
	var idx uint32
	ret := vk.AcquireNextImage(c.device, c.swapchain, vk.MaxUint64,
		c.imageAcquiredSemaphores[c.frameIndex], vk.NullFence, &idx)
	imageIndex = int(idx)
	if c.onInvalidate != nil {
		orPanic(c.onInvalidate(imageIndex))
	}
	switch ret {
	case vk.ErrorOutOfDate:
		c.frameIndex++
		c.frameIndex = c.frameIndex % c.frameLag
		c.prepareSwapchain(c.platform.PhysicalDevice(),
			c.platform.Surface(), c.SwapchainDimensions())
		c.prepare(true)
		outdated = true
		return
	case vk.Suboptimal, vk.Success:
	default:
		orPanic(NewError(ret))
	}

	graphicsQueue := c.platform.GraphicsQueue()
	var nullFence vk.Fence
	ret = vk.QueueSubmit(graphicsQueue, 1, []vk.SubmitInfo{{
		SType: vk.StructureTypeSubmitInfo,
		PWaitDstStageMask: []vk.PipelineStageFlags{
			vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		},
		WaitSemaphoreCount: 1,
		PWaitSemaphores: []vk.Semaphore{
			c.imageAcquiredSemaphores[c.frameIndex],
		},
		CommandBufferCount: 1,
		PCommandBuffers: []vk.CommandBuffer{
			c.swapchainImageResources[idx].cmd,
		},
		SignalSemaphoreCount: 1,
		PSignalSemaphores: []vk.Semaphore{
			c.drawCompleteSemaphores[c.frameIndex],
		},
	}}, nullFence)
	orPanic(NewError(ret))

	if c.platform.HasSeparatePresentQueue() {
		presentQueue := c.platform.PresentQueue()

		var nullFence vk.Fence
		ret = vk.QueueSubmit(presentQueue, 1, []vk.SubmitInfo{{
			SType: vk.StructureTypeSubmitInfo,
			PWaitDstStageMask: []vk.PipelineStageFlags{
				vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			},
			WaitSemaphoreCount: 1,
			PWaitSemaphores: []vk.Semaphore{
				c.imageAcquiredSemaphores[c.frameIndex],
			},
			CommandBufferCount: 1,
			PCommandBuffers: []vk.CommandBuffer{
				c.swapchainImageResources[idx].graphicsToPresentCmd,
			},
			SignalSemaphoreCount: 1,
			PSignalSemaphores: []vk.Semaphore{
				c.imageOwnershipSemaphores[c.frameIndex],
			},
		}}, nullFence)
		orPanic(NewError(ret))
	}
	return
}

func (c *context) PresentImage(imageIdx int) (outdated bool, err error) {
	// If we are using separate queues we have to wait for image ownership,
	// otherwise wait for draw complete.
	var semaphore vk.Semaphore
	if c.platform.HasSeparatePresentQueue() {
		semaphore = c.imageOwnershipSemaphores[c.frameIndex]
	} else {
		semaphore = c.drawCompleteSemaphores[c.frameIndex]
	}
	presentQueue := c.platform.PresentQueue()
	ret := vk.QueuePresent(presentQueue, &vk.PresentInfo{
		SType:              vk.StructureTypePresentInfo,
		WaitSemaphoreCount: 1,
		PWaitSemaphores:    []vk.Semaphore{semaphore},
		SwapchainCount:     1,
		PSwapchains:        []vk.Swapchain{c.swapchain},
		PImageIndices:      []uint32{uint32(imageIdx)},
	})
	c.frameIndex++
	c.frameIndex = c.frameIndex % c.frameLag

	switch ret {
	case vk.ErrorOutOfDate:
		outdated = true
		return
	case vk.Suboptimal, vk.Success:
		return
	default:
		err = NewError(ret)
		return
	}
}

type SwapchainImageResources struct {
	image                vk.Image
	cmd                  vk.CommandBuffer
	graphicsToPresentCmd vk.CommandBuffer

	view          vk.ImageView
	framebuffer   vk.Framebuffer
	descriptorSet vk.DescriptorSet

	uniformBuffer vk.Buffer
	uniformMemory vk.DeviceMemory
}

func (s *SwapchainImageResources) Destroy(dev vk.Device, cmdPool ...vk.CommandPool) {
	vk.DestroyFramebuffer(dev, s.framebuffer, nil)
	vk.DestroyImageView(dev, s.view, nil)
	if len(cmdPool) > 0 {
		vk.FreeCommandBuffers(dev, cmdPool[0], 1, []vk.CommandBuffer{
			s.cmd,
		})
	}
	vk.DestroyBuffer(dev, s.uniformBuffer, nil)
	vk.FreeMemory(dev, s.uniformMemory, nil)
}

func (s *SwapchainImageResources) SetImageOwnership(graphicsQueueFamilyIndex, presentQueueFamilyIndex uint32) {
	ret := vk.BeginCommandBuffer(s.graphicsToPresentCmd, &vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit),
	})
	orPanic(NewError(ret))

	vk.CmdPipelineBarrier(s.graphicsToPresentCmd,
		vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		0, 0, nil, 0, nil, 1, []vk.ImageMemoryBarrier{{
			SType:               vk.StructureTypeImageMemoryBarrier,
			DstAccessMask:       vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
			OldLayout:           vk.ImageLayoutPresentSrc,
			NewLayout:           vk.ImageLayoutPresentSrc,
			SrcQueueFamilyIndex: graphicsQueueFamilyIndex,
			DstQueueFamilyIndex: presentQueueFamilyIndex,
			Image:               s.image,

			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask: vk.ImageAspectFlags(vk.ImageAspectColorBit),
				LevelCount: 1,
				LayerCount: 1,
			},
		}})

	ret = vk.EndCommandBuffer(s.graphicsToPresentCmd)
	orPanic(NewError(ret))
}

func (s *SwapchainImageResources) SetUniformBuffer(buffer vk.Buffer, mem vk.DeviceMemory) {
	s.uniformBuffer = buffer
	s.uniformMemory = mem
}

func (s *SwapchainImageResources) Framebuffer() vk.Framebuffer {
	return s.framebuffer
}

func (s *SwapchainImageResources) SetFramebuffer(fb vk.Framebuffer) {
	s.framebuffer = fb
}

func (s *SwapchainImageResources) UniformBuffer() vk.Buffer {
	return s.uniformBuffer
}

func (s *SwapchainImageResources) UniformMemory() vk.DeviceMemory {
	return s.uniformMemory
}

func (s *SwapchainImageResources) CommandBuffer() vk.CommandBuffer {
	return s.cmd
}

func (s *SwapchainImageResources) Image() vk.Image {
	return s.image
}

func (s *SwapchainImageResources) View() vk.ImageView {
	return s.view
}

func (s *SwapchainImageResources) DescriptorSet() vk.DescriptorSet {
	return s.descriptorSet
}

func (s *SwapchainImageResources) SetDescriptorSet(set vk.DescriptorSet) {
	s.descriptorSet = set
}
