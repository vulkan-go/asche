package dieselvk

import (
	"fmt"

	vk "github.com/vulkan-go/vulkan"
)

type CoreRenderPass struct {
	renderPass []vk.RenderPass
}

func NewCoreRenderPass(passes int) *CoreRenderPass {
	var core CoreRenderPass
	core.renderPass = make([]vk.RenderPass, passes)
	return &core
}

//Creates default renderpass with a color and depth attachment, depth attachment is generated from the display
func (c *CoreRenderPass) CreateRenderPass(instance *CoreRenderInstance, display *CoreDisplay) {
	c.renderPass = make([]vk.RenderPass, 1)

	attachmentDescriptions := []vk.AttachmentDescription{
		{
			Flags:          vk.AttachmentDescriptionFlags(0),
			Format:         display.surface_format.Format,
			Samples:        vk.SampleCountFlagBits(vk.SampleCount1Bit),
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpStore,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutUndefined,
			FinalLayout:    vk.ImageLayoutPresentSrc},
		{
			Flags:          vk.AttachmentDescriptionFlags(0),
			Format:         display.depth_format,
			Samples:        vk.SampleCountFlagBits(vk.SampleCount1Bit),
			LoadOp:         vk.AttachmentLoadOpClear,
			StoreOp:        vk.AttachmentStoreOpStore,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutUndefined,
			FinalLayout:    vk.ImageLayoutPresentSrc},
	}

	//Setup Subpass Attachment References
	colorRef := vk.AttachmentReference{
		Attachment: 0,
		Layout:     vk.ImageLayoutColorAttachmentOptimal,
	}
	depthRef := vk.AttachmentReference{
		Attachment: 1,
		Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
	}
	colorReferences := []vk.AttachmentReference{
		colorRef,
	}
	depthReferences := []vk.AttachmentReference{
		depthRef,
	}

	//Configure Subpass bind to color and depth buffer for graphics
	subpass0 := vk.SubpassDescription{
		Flags:                   vk.SubpassDescriptionFlags(vk.SubpassDescriptionFlagBits(0x00000000)),
		PipelineBindPoint:       vk.PipelineBindPointGraphics,
		InputAttachmentCount:    0,
		PInputAttachments:       nil,
		ColorAttachmentCount:    1,
		PColorAttachments:       colorReferences,
		PResolveAttachments:     nil,
		PDepthStencilAttachment: &depthReferences[0],
		PreserveAttachmentCount: 0,
		PPreserveAttachments:    nil,
	}

	subpasses := []vk.SubpassDescription{
		subpass0,
	}

	subpass_dependencies := []vk.SubpassDependency{
		{
			SrcSubpass:      vk.MaxUint32,
			DstSubpass:      0,
			SrcStageMask:    vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit),
			DstStageMask:    vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			SrcAccessMask:   vk.AccessFlags(vk.AccessMemoryReadBit),
			DstAccessMask:   vk.AccessFlags(vk.AccessFlagBits(vk.AccessColorAttachmentReadBit) | vk.AccessFlagBits(vk.AccessColorAttachmentWriteBit)),
			DependencyFlags: vk.DependencyFlags(vk.DependencyFlagBits(vk.DependencyByRegionBit)),
		},
		{
			SrcSubpass:      0,
			DstSubpass:      vk.MaxUint32,
			SrcStageMask:    vk.PipelineStageFlags(vk.PipelineStageBottomOfPipeBit),
			DstStageMask:    vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
			SrcAccessMask:   vk.AccessFlags(vk.AccessMemoryReadBit),
			DstAccessMask:   vk.AccessFlags(vk.AccessFlagBits(vk.AccessColorAttachmentReadBit) | vk.AccessFlagBits(vk.AccessColorAttachmentWriteBit)),
			DependencyFlags: vk.DependencyFlags(vk.DependencyFlagBits(vk.DependencyByRegionBit)),
		},
	}

	//Create a default renderpass
	res := vk.CreateRenderPass(instance.logical_device.handle, &vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: uint32(len(attachmentDescriptions)),
		PAttachments:    attachmentDescriptions,
		SubpassCount:    uint32(len(subpasses)),
		PSubpasses:      subpasses,
		DependencyCount: uint32(len(subpass_dependencies)),
		PDependencies:   subpass_dependencies,
	}, nil, &c.renderPass[0])

	if res != vk.Success {
		Fatal(fmt.Errorf("Renderpass creation failed please enable vulkan layers for debugging\n"))
	}

}
