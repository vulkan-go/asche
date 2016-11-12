package asche

import vk "github.com/vulkan-go/vulkan"

// FenceManager keeps track of fences which in turn are used to keep track of GPU progress.
// The manager is not thread-safe and for rendering in multiple threads, multiple per-thread managers
// should be used.
type FenceManager struct {
	device vk.Device
	fences []vk.Fence
	count  uint32
}

func NewFenceManager(device vk.Device) *FenceManager {
	return &FenceManager{
		device: device,
	}
}

// Reset resets the state of fence manager. Waits for GPU to trigger all outstanding fences.
// After begin frame returns, it is safe to reuse or delete resources which were used previously.
func (f *FenceManager) Reset() {
	if f.count > 0 {
		vk.WaitForFences(f.device, f.count, f.fences, vk.True, vk.MaxUint64)
		vk.ResetFences(f.device, f.count, f.fences)
	}
	f.count = 0
}

func (f *FenceManager) NewFence() (vk.Fence, error) {
	if f.count < uint32(len(f.fences)) {
		f.count++
		return f.fences[f.count], nil
	}
	var fence vk.Fence
	ret := vk.CreateFence(f.device, &vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
	}, nil, &fence)
	if isError(ret) {
		return fence, newError(ret)
	}
	f.fences = append(f.fences, fence)
	f.count++
	return fence, nil
}

func (f *FenceManager) ActiveFences() []vk.Fence {
	return f.fences[:f.count]
}

func (f *FenceManager) Destroy() {
	f.Reset()
	for i := range f.fences {
		vk.DestroyFence(f.device, f.fences[i], nil)
	}
}

// CommandBufferManager allocates command buffers and recycles them for us.
// This gives us a convenient interface where we can request command buffers for use when rendering.
// The manager is not thread-safe and for rendering in multiple threads, multiple per-thread managers
// should be used.
type CommandBufferManager struct {
	device             vk.Device
	pool               vk.CommandPool
	buffers            []vk.CommandBuffer
	commandBufferLevel vk.CommandBufferLevel
	count              uint32
}

// NewCommandBufferManager creates a new instance of this manager. Device is the Vulkan device to use,
// bufferLevel is the command buffer level to use, either vk.CommandBufferLevelPrimary or vk.CommandBufferLevelSecondary.
// graphicsQueueIndex is the Vulkan queue family index for where we can submit graphics work.
func NewCommandBufferManager(device vk.Device,
	bufferLevel vk.CommandBufferLevel, graphicsQueueIndex uint32) (*CommandBufferManager, error) {

	var pool vk.CommandPool
	ret := vk.CreateCommandPool(device, &vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: graphicsQueueIndex,
		// ResetCommandBufferBit allows command buffers to be reset individually.
		Flags: vk.CommandPoolCreateFlags(vk.CommandPoolCreateResetCommandBufferBit),
	}, nil, &pool)

	if isError(ret) {
		return nil, newError(ret)
	}

	m := &CommandBufferManager{
		pool:               pool,
		device:             device,
		commandBufferLevel: bufferLevel,
	}
	return m, nil
}

// Reset resets the state of command buffer manager.
// When called, all managed command buffers are assumed to be recycleable.
func (c *CommandBufferManager) Reset() {
	c.count = 0
}

func (c *CommandBufferManager) Destroy() {
	vk.FreeCommandBuffers(c.device, c.pool, uint32(len(c.buffers)), c.buffers)
	vk.DestroyCommandPool(c.device, c.pool, nil)
}

// NewCommandBuffer returns a fresh or recycled command buffer which is in the reset state.
func (c *CommandBufferManager) NewCommandBuffer() (vk.CommandBuffer, error) {
	if c.count < uint32(len(c.buffers)) {
		c.count++
		buf := c.buffers[c.count]
		ret := vk.ResetCommandBuffer(buf,
			vk.CommandBufferResetFlags(vk.CommandBufferResetReleaseResourcesBit))
		if isError(ret) {
			return buf, newError(ret)
		}
		return buf, nil
	}
	c.count++
	c.buffers = append(c.buffers, nil)
	ret := vk.AllocateCommandBuffers(c.device, &vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        c.pool,
		Level:              c.commandBufferLevel,
		CommandBufferCount: 1,
	}, c.buffers[c.count:])
	err := newError(ret)
	return c.buffers[c.count], err
}
