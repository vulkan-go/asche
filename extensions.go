package dieselvk

import (
	"log"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

type Etxensions interface {
	HasRequired() (bool, []string)
	HasWanted() (bool, []string)
	GetExtensions() []string
}

// InstanceExtensions gets a list of instance extensions available on the platform.
func InstanceExtensions() (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateInstanceExtensionProperties("", &count, nil)
	Fatal(NewError(ret))
	list := make([]vk.ExtensionProperties, count)
	ret = vk.EnumerateInstanceExtensionProperties("", &count, list)
	Fatal(NewError(ret))
	for _, ext := range list {
		ext.Deref()
		names = append(names, vk.ToString(ext.ExtensionName[:]))
	}
	return names, err
}

// DeviceExtensions gets a list of instance extensions available on the provided physical device.
func DeviceExtensions(gpu vk.PhysicalDevice) (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateDeviceExtensionProperties(gpu, "", &count, nil)
	Fatal(NewError(ret))
	list := make([]vk.ExtensionProperties, count)
	ret = vk.EnumerateDeviceExtensionProperties(gpu, "", &count, list)
	Fatal(NewError(ret))
	for _, ext := range list {
		ext.Deref()
		names = append(names, vk.ToString(ext.ExtensionName[:]))
	}
	return names, err
}

// ValidationLayers gets a list of validation layers available on the platform.
func ValidationLayers() (names []string, err error) {
	defer checkErr(&err)

	var count uint32
	ret := vk.EnumerateInstanceLayerProperties(&count, nil)
	Fatal(NewError(ret))
	list := make([]vk.LayerProperties, count)
	ret = vk.EnumerateInstanceLayerProperties(&count, list)
	Fatal(NewError(ret))
	for _, layer := range list {
		layer.Deref()
		names = append(names, vk.ToString(layer.LayerName[:]))
	}
	return names, err
}

func FindRequiredMemoryType(props vk.PhysicalDeviceMemoryProperties,
	deviceRequirements, hostRequirements vk.MemoryPropertyFlagBits) (uint32, bool) {

	for i := uint32(0); i < vk.MaxMemoryTypes; i++ {
		if deviceRequirements&(vk.MemoryPropertyFlagBits(1)<<i) != 0 {
			props.MemoryTypes[i].Deref()
			flags := props.MemoryTypes[i].PropertyFlags
			if flags&vk.MemoryPropertyFlags(hostRequirements) != 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func FindRequiredMemoryTypeFallback(props vk.PhysicalDeviceMemoryProperties,
	deviceRequirements, hostRequirements vk.MemoryPropertyFlagBits) (uint32, bool) {

	for i := uint32(0); i < vk.MaxMemoryTypes; i++ {
		if deviceRequirements&(vk.MemoryPropertyFlagBits(1)<<i) != 0 {
			props.MemoryTypes[i].Deref()
			flags := props.MemoryTypes[i].PropertyFlags
			if flags&vk.MemoryPropertyFlags(hostRequirements) != 0 {
				return i, true
			}
		}
	}
	// Fallback to the first one available.
	if hostRequirements != 0 {
		return FindRequiredMemoryType(props, deviceRequirements, 0)
	}
	return 0, false
}

type Buffer struct {
	// device for destroy purposes.
	device vk.Device
	// Buffer is the buffer object.
	Buffer vk.Buffer
	// Memory is the device memory backing buffer object.
	Memory vk.DeviceMemory
}

func (b *Buffer) Destroy() {
	vk.FreeMemory(b.device, b.Memory, nil)
	vk.DestroyBuffer(b.device, b.Buffer, nil)
	b.device = nil
}

func CreateBuffer(device vk.Device, memProps vk.PhysicalDeviceMemoryProperties,
	data []byte, usage vk.BufferUsageFlagBits) *Buffer {

	var buffer vk.Buffer
	var memory vk.DeviceMemory
	ret := vk.CreateBuffer(device, &vk.BufferCreateInfo{
		SType: vk.StructureTypeBufferCreateInfo,
		Usage: vk.BufferUsageFlags(usage),
		Size:  vk.DeviceSize(len(data)),
	}, nil, &buffer)
	Fatal(NewError(ret))

	// Ask device about its memory requirements.
	var memReqs vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(device, buffer, &memReqs)
	memReqs.Deref()

	memType, ok := FindRequiredMemoryType(memProps, vk.MemoryPropertyFlagBits(memReqs.MemoryTypeBits),
		vk.MemoryPropertyHostVisibleBit|vk.MemoryPropertyHostCoherentBit)
	if !ok {
		log.Println("vulkan warning: failed to find required memory type")
	}

	// Allocate device memory and bind to the buffer.
	ret = vk.AllocateMemory(device, &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memType,
	}, nil, &memory)
	Fatal(NewError(ret), func() {
		vk.DestroyBuffer(device, buffer, nil)
	})
	vk.BindBufferMemory(device, buffer, memory, 0)
	b := &Buffer{
		device: device,
		Buffer: buffer,
		Memory: memory,
	}

	// Map the memory and dump data in there.
	if len(data) > 0 {
		var pData unsafe.Pointer
		ret := vk.MapMemory(device, memory, 0, vk.DeviceSize(len(data)), 0, &pData)
		if isError(ret) {
			log.Printf("vulkan warning: failed to map device memory for data (len=%d)", len(data))
			return b
		}
		n := vk.Memcopy(pData, data)
		if n != len(data) {
			log.Printf("vulkan warning: failed to copy data, %d != %d", n, len(data))
		}
		vk.UnmapMemory(device, memory)
	}
	return b
}

func LoadShaderModule(device vk.Device, data []byte) (vk.ShaderModule, error) {
	var module vk.ShaderModule
	ret := vk.CreateShaderModule(device, &vk.ShaderModuleCreateInfo{
		SType:    vk.StructureTypeShaderModuleCreateInfo,
		CodeSize: uint(len(data)),
		PCode:    sliceUint32(data),
	}, nil, &module)
	if isError(ret) {
		return vk.NullShaderModule, NewError(ret)
	}
	return module, nil
}
