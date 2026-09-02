package main

import (
	pgs "github.com/lyft/protoc-gen-star/v2"
	pgsgo "github.com/lyft/protoc-gen-star/v2/lang/go"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	optionalFeature := uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

	pgs.Init(pgs.DebugEnv("DEBUG_PGR"), pgs.SupportedFeatures(&optionalFeature)).
		RegisterModule(Redactor()).
		RegisterPostProcessor(pgsgo.GoFmt()).
		Render()
}
