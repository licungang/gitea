// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	process_module "code.gitea.io/gitea/modules/process"
)

// Processes prints out the processes
func Processes(ctx *context.PrivateContext) {
	pid := ctx.FormString("cancel-pid")
	if pid != "" {
		process_module.GetManager().Cancel(process_module.IDType(pid))
		runtime.Gosched()
		time.Sleep(100 * time.Millisecond)
	}

	flat := ctx.FormBool("flat")
	noSystem := ctx.FormBool("no-system")
	stacktraces := ctx.FormBool("stacktraces")
	json := ctx.FormBool("json")

	var processes []*process_module.Process
	goroutineCount := int64(0)
	var processCount int
	var err error
	if stacktraces {
		processes, processCount, goroutineCount, err = process_module.GetManager().ProcessStacktraces(flat, noSystem)
		if err != nil {
			log.Error("Unable to get stacktrace: %v", err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to get stacktraces: %v", err),
			})
			return
		}
	} else {
		processes, processCount = process_module.GetManager().Processes(flat, noSystem)
	}

	if json {
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"TotalNumberOfGoroutines": goroutineCount,
			"TotalNumberOfProcesses":  processCount,
			"Processes":               processes,
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	ctx.Resp.WriteHeader(http.StatusOK)

	if err := process_module.WriteProcesses(ctx.Resp, processes, processCount, goroutineCount, "", flat); err != nil {
		log.Error("Unable to write out process stacktrace: %v", err)
		if !ctx.Written() {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to get stacktraces: %v", err),
			})
		}
		return
	}
}
