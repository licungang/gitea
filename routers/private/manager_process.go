// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	process_module "code.gitea.io/gitea/modules/process"
)

// Processes prints out the processes
func Processes(ctx *context.PrivateContext) {
	flat := ctx.FormBool("flat")
	requestsOnly := ctx.FormBool("requests-only")
	stacktraces := ctx.FormBool("stacktraces")
	json := ctx.FormBool("json")

	var processes []*process_module.Process
	count := int64(0)
	var err error
	if stacktraces {
		processes, count, err = process_module.GetManager().ProcessStacktraces(flat, requestsOnly)
		if err != nil {
			log.Error("Unable to get stacktrace: %v", err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to get stacktraces: %v", err),
			})
			return
		}
	} else {
		processes = process_module.GetManager().Processes(!flat, requestsOnly, func() {})
	}

	if json {
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"TotalNumberOfGoroutines": count,
			"Processes":               processes,
		})
		return
	}

	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	ctx.Resp.WriteHeader(http.StatusOK)

	if err := writeProcesses(ctx.Resp, processes, count, "", flat); err != nil {
		log.Error("Unable to write out process stacktrace: %v", err)
		if !ctx.Written() {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to get stacktraces: %v", err),
			})
		}
		return
	}
}

func writeProcesses(out io.Writer, processes []*process_module.Process, numberOfGoroutines int64, indent string, flat bool) error {
	if numberOfGoroutines > 0 {
		if _, err := fmt.Fprintf(out, "%sNumber of goroutines: %d\n", indent, numberOfGoroutines); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "%sProcess: %d\n", indent, len(processes)); err != nil {
		return err
	}
	if len(processes) > 0 {
		if err := writeProcess(out, processes[0], "  ", flat); err != nil {
			return err
		}
	}
	if len(processes) > 1 {
		for _, process := range processes[1:] {
			if _, err := fmt.Fprintf(out, "%s  | \n", indent); err != nil {
				return err
			}
			if err := writeProcess(out, process, "  ", flat); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeProcess(out io.Writer, process *process_module.Process, indent string, flat bool) error {
	sb := &bytes.Buffer{}
	if flat {
		if process.ParentPID != "" {
			_, _ = fmt.Fprintf(sb, "%s+ PID: %s\t\tType: %s\n", indent, process.PID, process.Type)
		} else {
			_, _ = fmt.Fprintf(sb, "%s+ PID: %s:%s\tType: %s\n", indent, process.ParentPID, process.PID, process.Type)
		}
	} else {
		_, _ = fmt.Fprintf(sb, "%s+ PID: %s\tType: %s\n", indent, process.PID, process.Type)
	}
	indent += "| "

	_, _ = fmt.Fprintf(sb, "%sDescription: %s\n", indent, process.Description)
	_, _ = fmt.Fprintf(sb, "%sStart:       %s\n", indent, process.Start)

	if len(process.Stacks) > 0 {
		_, _ = fmt.Fprintf(sb, "%sGoroutines:\n", indent)
		for _, stack := range process.Stacks {
			indent := indent + "  "
			_, _ = fmt.Fprintf(sb, "%s+ Description: %s", indent, stack.Description)
			if stack.Count > 1 {
				_, _ = fmt.Fprintf(sb, "* %d", stack.Count)
			}
			_, _ = fmt.Fprintf(sb, "\n")
			indent += "| "
			if len(stack.Labels) > 0 {
				_, _ = fmt.Fprintf(sb, "%sLabels:      %q:%q", indent, stack.Labels[0].Name, stack.Labels[0].Value)

				if len(stack.Labels) > 1 {
					for _, label := range stack.Labels[1:] {
						_, _ = fmt.Fprintf(sb, ", %q:%q", label.Name, label.Value)
					}
				}
				_, _ = fmt.Fprintf(sb, "\n")
			}
			_, _ = fmt.Fprintf(sb, "%sStack:\n", indent)
			indent += "  "
			for _, entry := range stack.Entry {
				_, _ = fmt.Fprintf(sb, "%s+ %s\n", indent, entry.Function)
				_, _ = fmt.Fprintf(sb, "%s| %s:%d\n", indent, entry.File, entry.Line)
			}
		}
	}
	if _, err := out.Write(sb.Bytes()); err != nil {
		return err
	}
	sb.Reset()
	if len(process.Children) > 0 {
		if _, err := fmt.Fprintf(out, "%sChildren:\n", indent); err != nil {
			return err
		}
		for _, child := range process.Children {
			if err := writeProcess(out, child, indent+"  ", flat); err != nil {
				return err
			}
		}
	}
	return nil
}
