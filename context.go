package asche

import vk "github.com/vulkan-go/vulkan"

type Context interface {
	// OnPlatformUpdate sould be called when platform either initializes itself
	// or the swapchain has been recreated.
	OnPlatformUpdate(platform Platform) error
	// SetRenderingThreadCount sets the number of worker threads which can use secondary command buffers.
	// This call is blocking and will wait for all GPU work to complete before resizing.
	SetRenderingThreadCount(count uint)
	// Submit submits a command buffer to the queue.
	Submit(cmd vk.CommandBuffer)
	// SubmitSwapchain submits a command buffer to the queue which renders to the swapchain image.
	// The difference between this and Submit is that extra semaphores might be added to
	// the vk.QueueSubmit call depending on what was passed in to BeginFrame by the platform.
	SubmitSwapchain(cmd vk.CommandBuffer)
	// Device gets the Vulkan device assigned to the context.
	Device() vk.Device
	// Queue gets the Vulkan graphics queue assigned to the context.
	Queue() vk.Queue
	// Platform gets the current platform.
	Platform() Platform
	// NewPrimaryCommandBuffer gets a new or reset primary command buffer.
	//
	// The lifetime of this command buffer is only for the current frame.
	// It must be submitted in the same frame that the application obtains the command buffer.
	NewPrimaryCommandBuffer() (vk.CommandBuffer, error)
	// NewSecondaryCommandBuffer gets a new or reset secondary command buffer,
	// suitable for multithreaded rendering.
	//
	// The lifetime of this command buffer is only for the current frame.
	// It must be submitted in the same frame that the application obtains the command buffer.
	//
	// Parameter threadIndex specified the thread worker index in range [0, N)
	// which will be rendering using this secondary buffer.
	//
	// It is a race condition for two threads to use a command buffer which was obtained from the
	// same threadIndex. In order to use secondary command buffers, the application must call
	// SetRenderingThreadCount first.
	NewSecondaryCommandBuffer(threadIndex uint) (vk.CommandBuffer, error)
	// BeginFrame begins a frame.
	//
	// swapchainIdx is the swapchain index which will be rendered into this frame.
	//
	// When submitting command buffers using SubmitSwapchain,
	// use the acquireSemaphore as a wait semaphore in vk.QueueSubmit
	// to wait for the swapchain to become ready before rendering begins on GPU.
	// May be vk.NullSemaphore in case no waiting is required by the platform.
	//
	// Use releaseSemaphore to signal the swapchain that rendering has completed.
	// This semaphore is passed in as a signal semaphore in vk.QueueSubmit.
	// May be vk.NullSemaphore in case no waiting is required by the platform.
	//
	// It is the applications responsibility to add the appropriate vk.CmdPipelineBarrier calls
	// to ensure that the backbuffer is ready to be presented before the releaseSemaphore is signalled.
	BeginFrame(swapchainIdx uint32, acquireSemaphore, releaseSemaphore vk.Semaphore)
}

type context struct {
	platform             Platform
	device               vk.Device
	queue                vk.Queue
	swapchainIndex       uint32
	renderingThreadCount uint
	perFrameCtxs         []*perFrameCtx
}

func (c *context) destroy() {
	c.platform = nil
	for i := range c.perFrameCtxs {
		c.perFrameCtxs[i].Destroy()
	}
	c.perFrameCtxs = nil
}

func (c *context) Device() vk.Device {
	return c.device
}

func (c *context) Queue() vk.Queue {
	return c.queue
}

func (c *context) Platform() Platform {
	return c.platform
}

func (c *context) NewPrimaryCommandBuffer() (vk.CommandBuffer, error) {
	return c.perFrameCtxs[c.swapchainIndex].commandManager.NewCommandBuffer()
}

func (c *context) NewSecondaryCommandBuffer(threadIndex uint) (vk.CommandBuffer, error) {
	return c.perFrameCtxs[c.swapchainIndex].secondaryCommandManagers[threadIndex].NewCommandBuffer()
}

func (c *context) BeginFrame(swapchainIdx uint32, acquireSemaphore, releaseSemaphore vk.Semaphore) {
	c.swapchainIndex = swapchainIdx
	c.perFrameCtxs[c.swapchainIndex].Reset()
	c.perFrameCtxs[c.swapchainIndex].SetSwapchainSemaphores(acquireSemaphore, releaseSemaphore)
}

func (c *context) getFenceManager() *FenceManager {
	return c.perFrameCtxs[c.swapchainIndex].fenceManager
}

func (c *context) getSwapchainAcquireSemaphore() vk.Semaphore {
	return c.perFrameCtxs[c.swapchainIndex].swapchainAcquireSemaphore
}

func (c *context) getSwapchainReleaseSemaphore() vk.Semaphore {
	return c.perFrameCtxs[c.swapchainIndex].swapchainReleaseSemaphore
}

func (c *context) OnPlatformUpdate(platform Platform) (err error) {
	defer checkErr(&err)
	c.device = platform.Device()
	c.queue = platform.Queue()
	c.platform = platform
	vk.QueueWaitIdle(c.queue)

	// Initialize per-frame resources.
	// Every swapchain image has its own command pool and fence manager.
	// This makes it very easy to keep track of when we can reset command buffers and such.
	for i := range c.perFrameCtxs {
		c.perFrameCtxs[i].Destroy()
	}
	c.perFrameCtxs = c.perFrameCtxs[:0]
	images := platform.SwapchainImagesCount()
	queueIdx := platform.GraphicsQueueIndex()
	for i := uint32(0); i < images; i++ {
		c.perFrameCtxs = append(c.perFrameCtxs, newPerFrameCtx(c.device, queueIdx))
	}
	c.SetRenderingThreadCount(c.renderingThreadCount)
	return nil
}

func (c *context) SetRenderingThreadCount(count uint) {
	vk.QueueWaitIdle(c.queue)
	for i := range c.perFrameCtxs {
		c.perFrameCtxs[i].SetSecondaryCommandManagersCount(count)
	}
	c.renderingThreadCount = count
}

func (c *context) Submit(cmd vk.CommandBuffer) {
	c.submitCommandBuffer(cmd, vk.NullSemaphore, vk.NullSemaphore)
}

func (c *context) SubmitSwapchain(cmd vk.CommandBuffer) {
	c.submitCommandBuffer(cmd,
		c.perFrameCtxs[c.swapchainIndex].swapchainAcquireSemaphore,
		c.perFrameCtxs[c.swapchainIndex].swapchainReleaseSemaphore,
	)
}

func (c *context) submitCommandBuffer(cmd vk.CommandBuffer,
	acquireSemaphore, releaseSemaphore vk.Semaphore) {
	// All queue submissions get a fence that CPU will wait
	// on for synchronization purposes.
	fence, err := c.perFrameCtxs[c.swapchainIndex].fenceManager.NewFence()
	orPanic(err)

	submitInfos := []vk.SubmitInfo{{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers: []vk.CommandBuffer{
			cmd,
		},
	}}
	if acquireSemaphore != vk.NullSemaphore {
		submitInfos[0].WaitSemaphoreCount = 1
		submitInfos[0].PWaitSemaphores = []vk.Semaphore{
			acquireSemaphore,
		}
	}
	if releaseSemaphore != vk.NullSemaphore {
		submitInfos[0].SignalSemaphoreCount = 1
		submitInfos[0].PSignalSemaphores = []vk.Semaphore{
			releaseSemaphore,
		}
	}
	// PWaitDstStageMask is a pointer to an array of pipeline
	// stages at which each corresponding semaphore wait will occur.
	submitInfos[0].PWaitDstStageMask = []vk.PipelineStageFlags{
		vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
	}
	ret := vk.QueueSubmit(c.queue, 1, submitInfos, fence)
	orPanic(newError(ret))
}

type perFrameCtx struct {
	device         vk.Device
	fenceManager   *FenceManager
	commandManager *CommandBufferManager

	secondaryCommandManagers  []*CommandBufferManager
	swapchainAcquireSemaphore vk.Semaphore
	swapchainReleaseSemaphore vk.Semaphore

	queueIndex uint
}

func newPerFrameCtx(device vk.Device, graphicsQueueIndex uint32) *perFrameCtx {
	m, err := NewCommandBufferManager(device, vk.CommandBufferLevelPrimary, graphicsQueueIndex)
	orPanic(err)
	return &perFrameCtx{
		device:         device,
		fenceManager:   NewFenceManager(device),
		commandManager: m,
		queueIndex:     uint(graphicsQueueIndex),
	}
}

func (p *perFrameCtx) Reset() {
	p.fenceManager.Reset()
	p.commandManager.Reset()
	for i := range p.secondaryCommandManagers {
		p.secondaryCommandManagers[i].Reset()
	}
}

func (p *perFrameCtx) Destroy() {
	p.fenceManager.Destroy()
	p.commandManager.Destroy()
	if p.swapchainAcquireSemaphore != vk.NullSemaphore {
		vk.DestroySemaphore(p.device, p.swapchainAcquireSemaphore, nil)
	}
	if p.swapchainReleaseSemaphore != vk.NullSemaphore {
		vk.DestroySemaphore(p.device, p.swapchainReleaseSemaphore, nil)
	}
}

func (p *perFrameCtx) SetSwapchainSemaphores(acquireSemaphore, releaseSemaphore vk.Semaphore) {
	if p.swapchainAcquireSemaphore != vk.NullSemaphore {
		vk.DestroySemaphore(p.device, p.swapchainAcquireSemaphore, nil)
		p.swapchainAcquireSemaphore = acquireSemaphore
	}
	if p.swapchainReleaseSemaphore != vk.NullSemaphore {
		vk.DestroySemaphore(p.device, p.swapchainReleaseSemaphore, nil)
		p.swapchainReleaseSemaphore = releaseSemaphore
	}
}

func (p *perFrameCtx) SetSecondaryCommandManagersCount(count uint) error {
	for i := range p.secondaryCommandManagers {
		p.secondaryCommandManagers[i].Destroy()
	}
	p.secondaryCommandManagers = p.secondaryCommandManagers[:0]
	for i := uint(0); i < count; i++ {
		m, err := NewCommandBufferManager(p.device, vk.CommandBufferLevelSecondary, uint32(p.queueIndex))
		if err != nil {
			return err
		}
		p.secondaryCommandManagers = append(p.secondaryCommandManagers, m)
	}
	return nil
}
