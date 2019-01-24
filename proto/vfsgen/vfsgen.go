package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/shurcooL/vfsgen"
)

func main() {

	dir := flag.String("dir", "", "The directory to recursively generate vfs / embedded-bindata for")
	outfile := flag.String("outfile", "", "The file path and name (include extension) to output the generated file")
	pkg := flag.String("pkg", "", "The package name to give the vfs file")
	tags := flag.String("tags", "", "The build tags to give the vfs generation")
	variable := flag.String("variable", "", "The variable name to give the vfs (start with a capital letter if you want it exported)")
	comment := flag.String("comment", "", "The comment to give the variable")

	flag.Parse()

	fmt.Printf("vfsgen for directory: %s; output to: %s; package name: %s; build tags: %s; variable name: %s; comment: %s\n", *dir, *outfile, *pkg, *tags, *variable, *comment)
	err := vfsgen.Generate(http.Dir(*dir), vfsgen.Options{

		// Filename of the generated Go code output (including extension)
		Filename: *outfile,

		// PackageName is the name of the package in the generated code
		PackageName: *pkg,

		// BuildTags are optional build tags to give the generated code
		BuildTags: *tags,

		// VariableName is the name of the http.FileSystem variable in the generated code
		VariableName: *variable,

		// VariableComment is the comment of the http.FileSystem variable in the generated code
		VariableComment: *comment,
	})

	if err != nil {
		panic(err)
	}
}
