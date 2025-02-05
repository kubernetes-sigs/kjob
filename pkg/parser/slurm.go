/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parser

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/kjob/apis/v1alpha1"
)

const (
	slurmDirective  = "#SBATCH"
	srunCommand     = "srun"
	sbatchCommand   = "sbatch"
	sallocCommand   = "salloc"
	waitCommand     = "wait"
	sinfoCommand    = "sinfo"
	scontrolCommand = "scontrol"
	squeueCommand   = "squeue"
	scancelCommand  = "scancel"
)

var (
	longFlagFormat       = regexp.MustCompile(`(\w+[\w+-]*)=(\S+)`)
	shortFlagFormat      = regexp.MustCompile(`-(\w)\s+(\S+)`)
	envVarFormat         = regexp.MustCompile(`^([A-Z0-9_]+)=(?:"([^"]*)"|([^"\s]+))$`)
	notSupportedCommands = []string{sbatchCommand, sallocCommand, waitCommand, sinfoCommand, scontrolCommand, squeueCommand, scancelCommand}
)

type ParsedSlurmFlags struct {
	Array       string
	CpusPerTask *resource.Quantity
	GpusPerTask map[string]*resource.Quantity
	MemPerNode  *resource.Quantity
	MemPerTask  *resource.Quantity
	MemPerCPU   *resource.Quantity
	MemPerGPU   *resource.Quantity
	Nodes       *int32
	NTasks      *int32
	Output      string
	Error       string
	Input       string
	JobName     string
	Partition   string
	TimeLimit   string
}

func SlurmFlags(script string, ignoreUnknown bool) (ParsedSlurmFlags, error) {
	var flags ParsedSlurmFlags

	scanner := bufio.NewScanner(strings.NewReader(script))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		if !strings.HasPrefix(line, slurmDirective) {
			if strings.HasPrefix(line, "#") {
				continue
			}
			break
		}

		key, val := extractKeyValue(line)
		if len(key) == 0 || len(val) == 0 {
			continue
		}

		switch key {
		case "a", string(v1alpha1.ArrayFlag):
			flags.Array = val
		case string(v1alpha1.CpusPerTaskFlag):
			cpusPerTask, err := resource.ParseQuantity(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.CpusPerTask = ptr.To(cpusPerTask)
		case "e", string(v1alpha1.ErrorFlag):
			flags.Error = val
		case string(v1alpha1.GpusPerTaskFlag):
			gpusPerTask, err := GpusFlag(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.GpusPerTask = gpusPerTask
		case "i", string(v1alpha1.InputFlag):
			flags.Input = val
		case "J", string(v1alpha1.JobNameFlag):
			flags.JobName = val
		case string(v1alpha1.MemPerNodeFlag):
			memPerNode, err := resource.ParseQuantity(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.MemPerNode = ptr.To(memPerNode)
		case string(v1alpha1.MemPerCPUFlag):
			memPerCPU, err := resource.ParseQuantity(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.MemPerCPU = ptr.To(memPerCPU)
		case string(v1alpha1.MemPerGPUFlag):
			memPerGPU, err := resource.ParseQuantity(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.MemPerGPU = ptr.To(memPerGPU)
		case string(v1alpha1.MemPerTaskFlag):
			memPerTask, err := resource.ParseQuantity(val)
			if err != nil {
				return flags, fmt.Errorf("cannot parse '%s': %w", val, err)
			}
			flags.MemPerTask = ptr.To(memPerTask)
		case "N", string(v1alpha1.NodesFlag):
			intVal, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return flags, err
			}
			flags.Nodes = ptr.To(int32(intVal))
		case "n", string(v1alpha1.NTasksFlag):
			intVal, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return flags, err
			}
			flags.NTasks = ptr.To(int32(intVal))
		case "o", string(v1alpha1.OutputFlag):
			flags.Output = val
		case "p", string(v1alpha1.PartitionFlag):
			flags.Partition = val
		case "t", string(v1alpha1.TimeFlag):
			flags.TimeLimit = val
		default:
			if !ignoreUnknown {
				return ParsedSlurmFlags{}, fmt.Errorf("unknown flag: %s", key)
			}
		}
	}

	return flags, nil
}

func extractKeyValue(s string) (string, string) {
	matches := longFlagFormat.FindStringSubmatch(s)

	if len(matches) != 3 {
		matches = shortFlagFormat.FindStringSubmatch(s)
		if len(matches) != 3 {
			return "", ""
		}
	}

	return matches[1], matches[2]
}

func GpusFlag(val string) (map[string]*resource.Quantity, error) {
	gpus := make(map[string]*resource.Quantity)

	items := strings.Split(val, ",")
	for _, v := range items {
		gpu := strings.Split(v, ":")
		if len(gpu) != 2 {
			return nil, errors.New("invalid GPU format. It must be <type>:<number>")
		}

		name := gpu[0]
		quantity, err := resource.ParseQuantity(gpu[1])
		if err != nil {
			return nil, err
		}

		gpus[name] = &quantity
	}

	return gpus, nil
}

func SlurmValidateAndReplaceScript(script string, nTasks int32) (string, error) {
	if nTasks < 1 {
		return "", fmt.Errorf("invalid number of tasks: %d", nTasks)
	}

	var (
		sb          strings.Builder
		srunCount   int
		line        string
		needNewLine bool
	)

	lines := strings.Split(script, "\n")

	line = ""
	for i := 0; i < len(lines); i++ {
		if len(line) > 0 {
			line += " "
		}
		line += strings.TrimSpace(lines[i])

		// Convert multiline command to one line
		if strings.HasSuffix(line, " \\") {
			line = strings.TrimRight(line[:len(line)-2], " ")
			continue
		}

		if line == "" {
			sb.WriteByte('\n')
			continue
		}

		if nTasks > 1 && srunCount > 0 {
			return "", errors.New("multiple parallel tasks with more than one step are not supported yet")
		}

		if index := findCommand(line, srunCommand); index != -1 {
			srunCount++

			args := splitBySpaceWithIgnoreInQuotes(line[index:])[1:]

			srunFlagSet := pflag.NewFlagSet(srunCommand, pflag.ContinueOnError)
			srunFlagSet.ParseErrorsWhitelist.UnknownFlags = true

			err := srunFlagSet.Parse(args)
			if err != nil {
				return "", fmt.Errorf("invalid %q command: %w", line, err)
			}

			srunArgs := srunFlagSet.Args()
			if len(srunArgs) == 0 {
				return "", fmt.Errorf("invalid %q command: %s", line, "must specify at least one argument")
			}

			var envVars string
			if index > 0 {
				envVars = line[:index]
				// For now, we should only allow environment variables before srun command.
				err = validateEnvVars(envVars, srunCommand)
				if err != nil {
					return "", fmt.Errorf("invalid %q command: %w", line, err)
				}
			}

			line = fmt.Sprintf("%s%s", envVars, strings.Join(srunArgs, " "))
		}

		for _, command := range notSupportedCommands {
			if index := findCommand(line, command); index != -1 {
				return "", fmt.Errorf("%q command is not supported", command)
			}
		}

		if needNewLine {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
		needNewLine = true

		line = ""
	}

	return sb.String(), nil
}

func findCommand(input string, command string) int {
	input = strings.TrimSpace(input)

	index := strings.Index(input, command)
	if index == -1 {
		return -1
	}

	if index > 0 && input[index-1] != ' ' {
		return -1
	}

	withoutPrefix := input[index:]
	if withoutPrefix != command && !strings.HasPrefix(withoutPrefix, fmt.Sprintf("%s ", command)) {
		return -1
	}

	return index
}

// validateEnvVars validate environment variables before command.
// Possible values `FOO="bar" ` and `export FOO="bar" && ` or `export FOO="bar"; `
func validateEnvVars(input string, command string) error {
	parts := splitBySpaceWithIgnoreInQuotes(input)

	var export bool

	for _, part := range parts {
		length := len(part)

		switch {
		case part == "":
		case part == "export":
			if export {
				return errors.New("missed ';' or '&&' before \"export\" command")
			}
			export = true
		case part == "&&":
			export = false
		case strings.HasSuffix(part, ";") && !isEscaped(part, length-1):
			if length > 1 && part[length-2] == ';' {
				return errors.New("parse error near ';;'")
			}
			export = false
		default:
			matches := envVarFormat.FindStringSubmatch(part)
			if len(matches) == 0 {
				return fmt.Errorf("invalid %q environment variable", part)
			}
		}
	}

	if export {
		return fmt.Errorf("missed ';' or '&&' before %q command", command)
	}

	return nil
}

func splitBySpaceWithIgnoreInQuotes(str string) []string {
	var (
		parts          []string
		current        strings.Builder
		inSingleQuotes bool
		inDoubleQuotes bool
	)

	for i := 0; i < len(str); i++ {
		char := str[i]

		if !inSingleQuotes && char == '"' && !isEscaped(str, i) {
			inDoubleQuotes = !inDoubleQuotes
		}

		if !inDoubleQuotes && char == '\'' && !isEscaped(str, i) {
			inSingleQuotes = !inSingleQuotes
		}

		if char != ' ' || inSingleQuotes || inDoubleQuotes {
			current.WriteByte(char)
			if i < len(str)-1 {
				continue
			}
		}

		parts = append(parts, current.String())
		current.Reset()
	}

	return parts
}

func isEscaped(str string, index int) bool {
	var count int
	for index > 0 && str[index-1] == '\\' {
		count++
		index--
	}
	return count%2 != 0
}
