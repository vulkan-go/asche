package dieselvk

import (
	"fmt"

	vk "github.com/vulkan-go/vulkan"
)

type CorePool struct {
	pool vk.CommandPool
}

func NewCorePool(device *vk.Device, family_index uint32) (*CorePool, error) {
	var core CorePool
	var cmdPool vk.CommandPool

	ret := vk.CreateCommandPool(*device, &vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: family_index,
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateFlagBits(0x00000002)), //  VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT = 0x00000002,
	}, nil, &cmdPool)

	core.pool = cmdPool

	if ret != vk.Success {
		return &core, fmt.Errorf("Error creating command pool")
	}

	return &core, nil

}

func (c *CorePool) Destroy(device *vk.Device) {
	vk.DestroyCommandPool(*device, c.pool, nil)
}
