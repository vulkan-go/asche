package asche

import lin "github.com/xlab/linmath"

// VulkanProjectionMat converts an OpenGL style projection matrix to Vulkan style projection matrix.
// Vulkan has a topLeft clipSpace with [0, 1] depth range instead of [-1, 1].
//
// linmath outputs projection matrices in GL style clipSpace,
// perform a simple fixup step to change the projection to Vulkan style.
func VulkanProjectionMat(m *lin.Mat4x4, proj *lin.Mat4x4) {
	// Flip Y in clipspace. X = -1, Y = -1 is topLeft in Vulkan.
	m.Fill(1.0)
	m.ScaleAniso(m, 1.0, -1.0, 1.0)
	// Z depth is [0, 1] range instead of [-1, 1].
	m.ScaleAniso(m, 1.0, 1.0, 0.5)
	m.Translate(0.0, 0.0, 1.0)
	m.Mult(m, proj)
}
