//+build extras

package tools

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/Velocidex/ordereddict"
	"golang.org/x/net/context"
	"www.velocidex.com/golang/velociraptor/glob"
	vql_subsystem "www.velocidex.com/golang/velociraptor/vql"
	"www.velocidex.com/golang/velociraptor/vql/networking"
	"www.velocidex.com/golang/vfilter"
)

type httpUploadArgs struct {
	File     string `vfilter:"required,field=file,doc=The file to upload"`
	Name     string `vfilter:"optional,field=name,doc=The name of the file that should be stored on the server"`
	Accessor string `vfilter:"optional,field=accessor,doc=The accessor to use"`
	URI      string `vfilter:"optional,field=uri,doc=The URI to upload to"`
}

type httpUploadFunction struct{}

func (self *httpUploadFunction) Call(ctx context.Context,
	scope *vfilter.Scope,
	args *ordereddict.Dict) vfilter.Any {

	arg := &httpUploadArgs{}
	err := vfilter.ExtractArgs(scope, args, arg)
	if err != nil {
		scope.Log("upload_http: %s", err.Error())
		return vfilter.Null{}
	}

	accessor, err := glob.GetAccessor(arg.Accessor, ctx)
	if err != nil {
		scope.Log("upload_http: %v", err)
		return vfilter.Null{}
	}

	file, err := accessor.Open(arg.File)
	if err != nil {
		scope.Log("upload_http: Unable to open %s: %s",
			arg.File, err.Error())
		return &vfilter.Null{}
	}
	defer file.Close()

	if arg.Name == "" {
		arg.Name = arg.File
	}

	stat, err := file.Stat()
	if err != nil {
		scope.Log("upload_http: Unable to stat %s: %v",
			arg.File, err)
	} else if !stat.IsDir() {
		upload_response, err := upload_http(
			ctx, scope, file,
			arg.Name,
			arg.URI)
		if err != nil {
			scope.Log("upload_http: %v", err)
			return vfilter.Null{}
		}
		return upload_response
	}

	return vfilter.Null{}
}

func upload_http(ctx context.Context, scope *vfilter.Scope,
	reader io.Reader,
	name string,
	uri string) (
	*networking.UploadResponse, error) {

	scope.Log("upload_http: Uploading %v to %v", name, uri)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return &networking.UploadResponse{
			Error: err.Error(),
		}, err
	}
	io.Copy(part, reader)

	err = writer.Close()
	if err != nil {
		return &networking.UploadResponse{
			Error: err.Error(),
		}, err
	}

	request, err := http.NewRequest("POST", uri, body)
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	} else {
		return &networking.UploadResponse{
			Error: fmt.Sprintf("Error upload_http: %v , %v", resp.StatusCode, err.Error()),
		}, err
	}

	return &networking.UploadResponse{
		Path: name,
	}, nil
}

func (self httpUploadFunction) Info(
	scope *vfilter.Scope, type_map *vfilter.TypeMap) *vfilter.FunctionInfo {
	return &vfilter.FunctionInfo{
		Name:    "upload_http",
		Doc:     "Upload files to http.",
		ArgType: type_map.AddType(scope, &httpUploadArgs{}),
	}
}

func init() {
	vql_subsystem.RegisterFunction(&httpUploadFunction{})
}
