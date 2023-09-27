// Copyright 2020-2021 Tetrate
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

const clusterName = "cluster-manager"
const tickMilliseconds uint32 = 100

func main() {
	proxywasm.SetVMContext(&vmContext{})
}

type vmContext struct {
	// Embed the default VM context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultVMContext
}

// Override types.DefaultVMContext.
func (*vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	proxywasm.LogInfo("create new PluginContext")
	return &pluginContext{}
}

type pluginContext struct {
	// Embed the default plugin context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultPluginContext
	callBack func(numHeaders, bodySize, numTrailers int)
	queue    []string
}

func (ctx *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	if err := proxywasm.SetTickPeriodMilliSeconds(tickMilliseconds); err != nil {
		proxywasm.LogCriticalf("failed to set tick period: %v", err)
		return types.OnPluginStartStatusFailed
	}
	proxywasm.LogInfof("set tick period milliseconds: %d", tickMilliseconds)
	ctx.callBack = func(numHeaders, bodySize, numTrailers int) {
		b, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
		if err != nil {
			proxywasm.LogCriticalf("failed to get response body: %v", err)
		}
		proxywasm.LogInfof("http call resp %s", string(b))
	}
	return types.OnPluginStartStatusOK
}

func (ctx *pluginContext) OnTick() {
	if len(ctx.queue) > 0 {
		proxywasm.LogInfof("current item: %s", ctx.queue[0])
		ctx.queue = ctx.queue[1:]
	}

	if _, err := proxywasm.DispatchHttpCall(clusterName, [][2]string{
		{":path", "/ip"},
		{":method", "GET"},
		{":authority", ""}},
		nil, nil, 50000, ctx.callBack); err != nil {
		proxywasm.LogErrorf("call http err %v", err)
	}
}

// Override types.DefaultPluginContext.
func (ctx *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpContext{contextID: contextID, parent: ctx}
}

type httpContext struct {
	// Embed the default http context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultHttpContext
	// contextID is the unique identifier assigned to each httpContext.
	contextID uint32
	parent    *pluginContext
}

// Override types.DefaultHttpContext.
func (ctx *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	// On each request response, we dispatch the http calls `totalDispatchNum` times.
	// Note: DispatchHttpCall is asynchronously processed, so each loop is non-blocking.
	proxywasm.LogInfo("on response heaer")
	ctx.parent.queue = append(ctx.parent.queue, "response")
	// for i := 0; i < totalDispatchNum; i++ {
	// 	if _, err := proxywasm.DispatchHttpCall(clusterName, [][2]string{
	// 		{":path", "/ip"},
	// 		{":method", "GET"},
	// 		{":authority", ""}},
	// 		nil, nil, 50000, ctx.dispatchCallback); err != nil {
	// 		panic(err)
	// 	}
	// 	// Now we have made a dispatched request, so we record it.
	// }
	return types.ActionContinue
}

// dispatchCallback is the callback function called in response to the response arrival from the dispatched request.
func (ctx *httpContext) dispatchCallback(numHeaders, bodySize, numTrailers int) {
	b, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogCriticalf("failed to get response body: %v", err)
	}
	proxywasm.LogInfof("http call resp %s", string(b))
}
