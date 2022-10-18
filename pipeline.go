package dieselvk

import (
	"C"

	vk "github.com/vulkan-go/vulkan"
)

type CorePipeline struct {
	layouts   map[string]*vk.PipelineLayout
	pipelines map[string]*vk.Pipeline
}

func NewCorePipeline() *CorePipeline {
	var core CorePipeline
	core.layouts = make(map[string]*vk.PipelineLayout, 4)
	core.pipelines = make(map[string]*vk.Pipeline, 4)
	core.layouts["Primary"] = &vk.NullPipelineLayout
	core.pipelines["Primary"] = &vk.NullPipeline
	return &core
}

type PipelineBuilder struct {
	_shaderStages         []vk.PipelineShaderStageCreateInfo
	_vertexInputInfo      vk.PipelineVertexInputStateCreateInfo
	_inputAssembly        vk.PipelineInputAssemblyStateCreateInfo
	_viewport             vk.Viewport
	_scissor              vk.Rect2D
	_rasterizer           vk.PipelineRasterizationStateCreateInfo
	_colorBlendAttachment vk.PipelineColorBlendAttachmentState
	_multisampling        vk.PipelineMultisampleStateCreateInfo
	_pipelineLayout       vk.PipelineLayout
	_pipeline             vk.Pipeline
}

//Default Triangle Pipeline with vertex and frag shader //generalize for Multivariate pipelines
func NewPiplelineBuilder(instance *CoreRenderInstance, program *ShaderProgram) *PipelineBuilder {

	pb := PipelineBuilder{}

	//Shader Stages
	pb._shaderStages = make([]vk.PipelineShaderStageCreateInfo, 2)

	vx_module := program.vertex_shader_modules
	fg_module := program.fragment_shader_modules

	//Shader VertexStage
	vx_stage := vk.PipelineShaderStageCreateInfo{}
	vx_stage.SType = vk.StructureTypePipelineShaderStageCreateInfo
	vx_stage.PNext = nil
	vx_stage.Flags = vk.PipelineShaderStageCreateFlags(0)
	vx_stage.Stage = vk.ShaderStageFlagBits(vk.ShaderStageVertexBit)
	vx_stage.PName = safeString("main")
	vx_stage.Module = *vx_module

	//Shader Frags
	fg_stage := vk.PipelineShaderStageCreateInfo{}
	fg_stage.SType = vk.StructureTypePipelineShaderStageCreateInfo
	fg_stage.PNext = nil
	fg_stage.Stage = vk.ShaderStageFlagBits(vk.ShaderStageFragmentBit)
	fg_stage.Flags = vk.PipelineShaderStageCreateFlags(0)
	fg_stage.PName = safeString("main")
	fg_stage.Module = *fg_module

	pb._shaderStages[0] = vx_stage
	pb._shaderStages[1] = fg_stage

	//Vertex Info
	vert_input := vk.PipelineVertexInputStateCreateInfo{
		SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
		VertexBindingDescriptionCount:   0,
		VertexAttributeDescriptionCount: 0,
	}
	pb._vertexInputInfo = vert_input
	//Input Assembly
	assembly := vk.PipelineInputAssemblyStateCreateInfo{}
	assembly.SType = vk.StructureTypePipelineInputAssemblyStateCreateInfo
	assembly.PNext = nil
	assembly.Topology = vk.PrimitiveTopologyTriangleList
	assembly.PrimitiveRestartEnable = vk.False

	pb._inputAssembly = assembly

	//Rasterization CreatInfo
	rasterizer := vk.PipelineRasterizationStateCreateInfo{}
	rasterizer.SType = vk.StructureTypePipelineRasterizationStateCreateInfo
	rasterizer.PNext = nil
	rasterizer.DepthClampEnable = vk.False
	rasterizer.RasterizerDiscardEnable = vk.False //discards primitives before rasterization stage
	rasterizer.PolygonMode = vk.PolygonModeFill   //Fill and Wire
	rasterizer.CullMode = vk.CullModeFlags(vk.CullModeNone)
	rasterizer.FrontFace = vk.FrontFaceClockwise
	rasterizer.DepthBiasEnable = vk.False
	rasterizer.DepthBiasConstantFactor = 0.0
	rasterizer.DepthBiasClamp = 0.0
	rasterizer.DepthBiasSlopeFactor = 0.0
	rasterizer.LineWidth = 1.0

	pb._rasterizer = rasterizer

	//Multisample State
	mss := vk.PipelineMultisampleStateCreateInfo{}
	mss.SType = vk.StructureTypePipelineMultisampleStateCreateInfo
	mss.PNext = nil
	mss.SampleShadingEnable = vk.False
	mss.RasterizationSamples = vk.SampleCount1Bit
	mss.MinSampleShading = 1.0
	mss.PSampleMask = nil
	mss.AlphaToCoverageEnable = vk.False
	mss.AlphaToOneEnable = vk.False

	pb._multisampling = mss

	//Color Blend
	cbb := vk.PipelineColorBlendAttachmentState{}
	cbb.ColorWriteMask = vk.ColorComponentFlags(vk.ColorComponentRBit) | vk.ColorComponentFlags(vk.ColorComponentGBit) | vk.ColorComponentFlags(vk.ColorComponentBBit)
	cbb.BlendEnable = vk.False

	pb._colorBlendAttachment = cbb

	return &pb

}

func (p *PipelineBuilder) BuildPipeline(instance *CoreRenderInstance, renderpass_id string, display *CoreDisplay, layout *vk.PipelineLayout) *vk.Pipeline {

	viewports := []vk.Viewport{display.viewport}
	scissors := []vk.Rect2D{{Offset: vk.Offset2D{}, Extent: display.extent}}

	attachments := []vk.PipelineColorBlendAttachmentState{p._colorBlendAttachment}
	view_create := vk.PipelineViewportStateCreateInfo{}

	view_create.SType = vk.StructureTypePipelineViewportStateCreateInfo
	view_create.PNext = nil
	view_create.ViewportCount = 1
	view_create.PViewports = viewports
	view_create.PScissors = scissors
	view_create.ScissorCount = 1

	//Setup Dummy color blending. We aren't using transparent objects yet
	//the blending is just no blend, but we do write to the color attaachment
	blend_state := vk.PipelineColorBlendStateCreateInfo{}
	blend_state.SType = vk.StructureTypePipelineColorBlendStateCreateInfo
	blend_state.PNext = nil

	blend_state.LogicOpEnable = vk.False
	blend_state.LogicOp = vk.LogicOpCopy
	blend_state.AttachmentCount = 1
	blend_state.PAttachments = attachments

	//Pipeline Empty Layout ....if we need descriptor sets we need to move this to a core object
	depth_state := vk.PipelineDepthStencilStateCreateInfo{}
	depth_state.SType = vk.StructureTypePipelineDepthStencilStateCreateInfo
	depth_state.Flags = vk.PipelineDepthStencilStateCreateFlags(0)
	//Shaders stages

	pipeline_info := vk.GraphicsPipelineCreateInfo{}
	pipeline_info.SType = vk.StructureTypeGraphicsPipelineCreateInfo
	pipeline_info.PNext = nil
	pipeline_info.StageCount = 2
	pipeline_info.PStages = p._shaderStages
	pipeline_info.PVertexInputState = &p._vertexInputInfo
	pipeline_info.PInputAssemblyState = &p._inputAssembly

	pipeline_info.PViewportState = &view_create
	pipeline_info.PRasterizationState = &p._rasterizer
	pipeline_info.PMultisampleState = &p._multisampling
	pipeline_info.PColorBlendState = &blend_state
	pipeline_info.PDepthStencilState = &depth_state
	pipeline_info.Layout = *layout
	pipeline_info.RenderPass = instance.renderpasses[renderpass_id].renderPass[0]
	pipeline_info.Subpass = 0
	pipeline_info.BasePipelineHandle = nil

	//Build actual pipeline
	var pipelines = []vk.Pipeline{vk.NullPipeline}
	res := vk.CreateGraphicsPipelines(instance.logical_device.handle, nil, 1, []vk.GraphicsPipelineCreateInfo{pipeline_info}, nil, pipelines)
	if res != vk.Success {
		Fatal(NewError(res))
	}
	return &pipelines[0]

}
