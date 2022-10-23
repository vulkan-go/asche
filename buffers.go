package dieselvk

import (
	"fmt"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

type CoreBuffer struct {
	buffer          []vk.Buffer
	device_memory   []vk.DeviceMemory
	location        uint32
	descriptor_type uint32
	stage_flags     vk.ShaderStageFlags
	layout          vk.DescriptorSetLayout
	name            string
}

func NewCoreUniformBuffer(handle vk.Device, name string, bind_loc uint32, stage_flags vk.ShaderStageFlags, bytes_size int, frames int) CoreBuffer {
	core := CoreBuffer{}
	core.location = bind_loc
	core.descriptor_type = uint32(vk.DescriptorTypeUniformBuffer)
	core.stage_flags = stage_flags
	core.name = name
	core.buffer = make([]vk.Buffer, frames)
	core.device_memory = make([]vk.DeviceMemory, frames)

	ubo_layout := vk.DescriptorSetLayoutBinding{}
	ubo_layout.Binding = core.location
	ubo_layout.DescriptorCount = 1
	ubo_layout.DescriptorType = vk.DescriptorTypeUniformBuffer
	ubo_layout.StageFlags = core.stage_flags
	ubo_layout.PImmutableSamplers = nil

	bindings := []vk.DescriptorSetLayoutBinding{ubo_layout}

	ubo_create := vk.DescriptorSetLayoutCreateInfo{}
	ubo_create.SType = vk.StructureTypeDescriptorSetLayoutCreateInfo
	ubo_create.BindingCount = 1
	ubo_create.PBindings = bindings

	if vk.CreateDescriptorSetLayout(handle, &ubo_create, nil, &core.layout) != vk.Success {
		Fatal(fmt.Errorf("Failed to create uniform buffer object"))
	}

	dev_size := vk.DeviceSize(bytes_size)

	buffer_create := vk.BufferCreateInfo{}
	buffer_create.SType = vk.StructureTypeBufferCreateInfo
	buffer_create.Flags = vk.BufferCreateFlags(vk.BufferUsageVertexBufferBit)
	buffer_create.SharingMode = vk.SharingMode(vk.MemoryPropertyHostVisibleBit | vk.MemoryPropertyHostCoherentBit)
	buffer_create.Size = dev_size

	for i := 0; i < frames; i++ {
		vk.CreateBuffer(handle, &buffer_create, nil, &core.buffer[i])
	}

	//TODO CREATE MANAGING DESRIPTOR POOLS IN INSTANCE
	//

	return core

}

func (core *CoreBuffer) MapMemory(data *unsafe.Pointer, index int, instance *CoreRenderInstance) {
	vk.MapMemory(instance.logical_device.handle, core.device_memory[index], vk.DeviceSize(0), vk.DeviceSize(4),
		vk.MemoryMapFlags(vk.MemoryPropertyHostVisibleBit), data)
}
