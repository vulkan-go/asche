package dieselvk

import (
	vk "github.com/vulkan-go/vulkan"
)

//Device Queue properties is per device and constructed from the device
type CoreQueue struct {
	binded     []bool
	properties []vk.QueueFamilyProperties
	gpu        *vk.PhysicalDevice
	queues     []vk.Queue
}

//List queue properties available for a physical device and enable queue creation
func NewCoreQueue(gpu vk.PhysicalDevice, device_name string) *CoreQueue {
	var q CoreQueue
	var count uint32
	q.gpu = &gpu
	vk.GetPhysicalDeviceQueueFamilyProperties(gpu, &count, nil)
	q.properties = make([]vk.QueueFamilyProperties, count)
	q.binded = make([]bool, count)
	q.queues = make([]vk.Queue, count)
	vk.GetPhysicalDeviceQueueFamilyProperties(gpu, &count, q.properties)

	if count == 0 {
		return nil
	}

	return &q
}

//Gets Devices Create Info with a single queue count. Extend this implementation if you need more than one queue per
//queue family.
func (q *CoreQueue) GetCreateInfos() []vk.DeviceQueueCreateInfo {
	count := len(q.properties)
	infos := make([]vk.DeviceQueueCreateInfo, count)
	priority := float32(0.5)
	for index := 0; index < count; index++ {
		infos[index] = vk.DeviceQueueCreateInfo{}
		infos[index].SType = vk.StructureTypeDeviceQueueCreateInfo
		infos[index].QueueFamilyIndex = uint32(index)
		infos[index].PQueuePriorities = []float32{priority}
		infos[index].QueueCount = 1
	}
	return infos
}

//Checks if device is suitable for queue operations
func (q *CoreQueue) IsDeviceSuitable(flag_bits uint32) bool {
	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(flag_bits)
		if flag == vk.QueueFlags(flag_bits) {
			return true
		}
	}
	return false
}

//Passes a device a initiates the actual queue objects. Must call after logical device is established
func (q *CoreQueue) CreateQueues(device vk.Device) {
	for index := 0; index < len(q.properties); index++ {
		vk.GetDeviceQueue(device, uint32(index), 0, &q.queues[index])
	}
}

//Finds a suitable queue mode given flag bits does not check if the queue is already being used with an instance
func (q *CoreQueue) FindSuitableQueue(flag_bits uint32) (bool, int) {
	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(flag_bits)
		if flag == vk.QueueFlags(flag_bits) {
			return true, index
		}
	}
	return false, 0
}

//Finds a suitable queue mode given flag bits does not check if the queue is already being used with an instance
func (q *CoreQueue) FindSuitableUnboundQueue(flag_bits uint32) (bool, int) {
	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(flag_bits)
		if flag == vk.QueueFlags(flag_bits) && !q.binded[index] {
			return true, index
		}
	}
	return false, 0
}

//Finds and binds a suitable queue given the flag bits. Does not check if queue is already bound
//if you need to use a unbound queue use BindSuitableUnboundQueue()returns nil if no queue is found
func (q *CoreQueue) BindSuitableQueue(device vk.Device, flag_bits uint32, queue_instance uint32) (bool, *vk.Queue) {

	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(flag_bits)
		if flag == vk.QueueFlags(flag_bits) {
			return true, &q.queues[index]
		}
	}
	return false, nil
}

//Finds and binds a suitable queue given the flag bits. Does not check if queue is already bound
//if you need to use a unbound queue use BindSuitableUnboundQueue()returns nil if no queue is found
func (q *CoreQueue) BindSuitableUnboundQueue(device vk.Device, flag_bits uint32, queue_instance uint32) (bool, *vk.Queue) {

	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(flag_bits)
		if flag == vk.QueueFlags(flag_bits) && !q.binded[index] {
			return true, &q.queues[index]
		}
	}
	return false, nil
}

//Function to gather graphics / present primary queue
func (q *CoreQueue) BindGraphicsQueue(device vk.Device) (bool, *vk.Queue, int) {
	for index := 0; index < len(q.properties); index++ {
		queue := q.properties[index]
		queue.Deref()
		flag := queue.QueueFlags & vk.QueueFlags(vk.QueueGraphicsBit)
		if flag == vk.QueueFlags(vk.QueueGraphicsBit) && !q.binded[index] {
			return true, &q.queues[index], index
		}
	}
	return false, nil, 0
}

//Checks if queue is already being used in a specific context. This
//can be used when a separate queue is desired for example for seperate
//instances
func (q *CoreQueue) IsBound(index int) bool {
	return q.binded[index]
}
