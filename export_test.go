package markdown

// DistributeWidths exposes the column width distribution to the external
// test package.
var DistributeWidths = distributeWidths

// CodeOffset exposes how far a code block has been scrolled horizontally, in
// pixels, to the external test package. It is what a fence's own scrolling
// can be asserted on without going through the pixels.
func CodeOffset(d *Document, b *CodeBlock) int { return d.codeState(b).Offset() }
