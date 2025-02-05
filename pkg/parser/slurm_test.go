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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestParseSlurmOptions(t *testing.T) {
	testCases := map[string]struct {
		script        string
		ignoreUnknown bool
		want          ParsedSlurmFlags
		wantErr       string
	}{
		"should parse simple script": {
			script: `#!/bin/bash
#SBATCH --job-name=single_Cpu
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=1

sleep 30
echo "hello"`,
			want: ParsedSlurmFlags{
				JobName:     "single_Cpu",
				NTasks:      ptr.To[int32](1),
				CpusPerTask: ptr.To(resource.MustParse("1")),
			},
		},
		"should parse script with short flags": {
			script: `#SBATCH -J serial_Test_Job
#SBATCH -n 1
#SBATCH -o output.%j
#SBATCH -e error.%j

./myexec
exit 0`,
			want: ParsedSlurmFlags{
				JobName: "serial_Test_Job",
				NTasks:  ptr.To[int32](1),
				Output:  "output.%j",
				Error:   "error.%j",
			},
		},
		"should parse script with comments": {
			script: `#!/bin/bash
# Job name
#SBATCH --job-name=job-array
# Defines a job array from task ID 1 to 20
#SBATCH --array=1-20
# Number of tasks (in this case, one task per array element)
#SBATCH -n 1
# Partition or queue name
#SBATCH --partition=shared                      
#SBATCH                           # This is an empty line to separate Slurm directives from the job commands

echo "Start Job $SLURM_ARRAY_TASK_ID on $HOSTNAME"  # Display job start information

sleep 10`,
			want: ParsedSlurmFlags{
				JobName:   "job-array",
				Array:     "1-20",
				NTasks:    ptr.To[int32](1),
				Partition: "shared",
			},
		},
		"should parse script and ignore unknown flags": {
			script: `#!/bin/bash
#SBATCH --job-name=my_job_name
#SBATCH --output=output.txt
#SBATCH --error=error.txt
#SBATCH --partition=partition_name
#SBATCH --nodes=1
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=1
#SBATCH --time=1:00:00
#SBATCH --mail-type=END
#SBATCH --mail-user=your@email.com
#SBATCH --unknown=unknown

python my_script.py`,
			ignoreUnknown: true,
			want: ParsedSlurmFlags{
				JobName:     "my_job_name",
				Output:      "output.txt",
				Error:       "error.txt",
				Partition:   "partition_name",
				Nodes:       ptr.To[int32](1),
				NTasks:      ptr.To[int32](1),
				CpusPerTask: ptr.To(resource.MustParse("1")),
				TimeLimit:   "1:00:00",
			},
		},
		"should fail due to unknown flags": {
			script: `#!/bin/bash
#SBATCH --job-name=my_job_name
#SBATCH --nodes=1
#SBATCH --cpus-per-task=1
#SBATCH --unknown=unknown

python my_script.py`,
			wantErr: "unknown flag: unknown",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := SlurmFlags(tc.script, tc.ignoreUnknown)

			var gotErrStr string
			if gotErr != nil {
				gotErrStr = gotErr.Error()
			}
			if diff := cmp.Diff(tc.wantErr, gotErrStr); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
				return
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected options (-want/+got)\n%s", diff)
			}
		})
	}
}

func TestSlurmValidateAndReplaceScript(t *testing.T) {
	testCases := map[string]struct {
		script  string
		nTasks  int32
		want    string
		wantErr string
	}{
		"nTasks less than 1": {
			script:  "#!/bin/bash",
			nTasks:  0,
			wantErr: "invalid number of tasks: 0",
		},
		"UC1. Run an interactive shell on a compute node": {
			script: "srun --cpus-per-task=4 --time=2:00:00 --mem=4000 /bin/bash",
			nTasks: 1,
			want:   "/bin/bash",
		},
		"UC2. Run an interactive job with multiple parallel tasks": {
			script: "srun -n 2 -c 2 --mem=8G -p compute -t 0-2 my_program",
			nTasks: 2,
			want:   "my_program",
		},
		"UC3. Run batch job with a single task": {
			script: `#!/bin/bash
#SBATCH --job-name=single_task_job
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=4G
#SBATCH --time=0-1
#SBATCH --partition=compute

srun -n 1 my_program < input,txt
`,
			nTasks: 1,
			want: `#!/bin/bash
#SBATCH --job-name=single_task_job
#SBATCH --ntasks=1
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=4G
#SBATCH --time=0-1
#SBATCH --partition=compute

my_program < input,txt
`,
		},
		"UC4. Run batch job with a multiple parallel tasks": {
			script: `#!/bin/bash
#SBATCH --job-name=parallel_task_job
#SBATCH --ntasks=8
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=4G
#SBATCH --time=0-1
#SBATCH --partition=compute

srun my_program
`,
			nTasks: 8,
			want: `#!/bin/bash
#SBATCH --job-name=parallel_task_job
#SBATCH --ntasks=8
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=4G
#SBATCH --time=0-1
#SBATCH --partition=compute

my_program
`,
		},
		"UC5. Run array job with a single task per job": {
			script: `#!/bin/bash
#SBATCH --job-name=array_job
#SBATCH --array=1-10
#SBATCH -n 1
#SBATCH -c 1
#SBATCH --mem=4G
#SBATCH -p compute
#SBATCH -t 0-1
#SBATCH -o array_job_%A_%a.out

input_file=input_${SLURM_ARRAY_TASK_ID}.txt

srun -n 1 my_program < $input_file
`,
			nTasks: 1,
			want: `#!/bin/bash
#SBATCH --job-name=array_job
#SBATCH --array=1-10
#SBATCH -n 1
#SBATCH -c 1
#SBATCH --mem=4G
#SBATCH -p compute
#SBATCH -t 0-1
#SBATCH -o array_job_%A_%a.out

input_file=input_${SLURM_ARRAY_TASK_ID}.txt

my_program < $input_file
`,
		},
		"UC6. Run multiple tasks in interactive mode within salloc": {
			script: `salloc -n 2 -c 2 --mem=8G -p compute -t 0-2

srun -n 1 my_program_1 &
srun -n 1 my_program_2 &
wait

srun my_program_3
exit
`,
			nTasks:  2,
			wantErr: "\"salloc\" command is not supported",
		},
		"UC7. Run batch job with multiple steps/parallel tasks": {
			script: `#!/bin/bash
#SBATCH --job-name=parallel_task_job
#SBATCH --ntasks=8
#SBATCH --cpus-per-task=1
#SBATCH --mem-per-cpu=4G
#SBATCH --time=0-1
#SBATCH --partition=compute

srun my_program_1
srun my_program_2

srun -n 4 -c 1 my_program_3 &
srun -n 4 -c 1 my_program_4 &
wait
`,
			nTasks:  8,
			wantErr: "multiple parallel tasks with more than one step are not supported yet",
		},
		"srun must specified at lease one argument": {
			script:  "#!/bin/bash\nsrun",
			nTasks:  1,
			wantErr: "invalid \"srun\" command: must specify at least one argument",
		},
		"unsupported wait command": {
			script:  "#!/bin/bash\nsrun my_program &\nwait",
			nTasks:  1,
			wantErr: "\"wait\" command is not supported",
		},
		"unsupported sinfo command": {
			script:  "#!/bin/bash\nsinfo",
			nTasks:  1,
			wantErr: "\"sinfo\" command is not supported",
		},
		"unsupported sinfo command with parameters": {
			script:  "#!/bin/bash\nsinfo -s",
			nTasks:  1,
			wantErr: "\"sinfo\" command is not supported",
		},
		"unsupported scontrol command": {
			script:  "#!/bin/bash\nscontrol -s",
			nTasks:  1,
			wantErr: "\"scontrol\" command is not supported",
		},
		"unsupported scontrol command with parameters": {
			script:  "#!/bin/bash\nscontrol create reservation",
			nTasks:  1,
			wantErr: "\"scontrol\" command is not supported",
		},
		"unsupported squeue command": {
			script:  "#!/bin/bash\nsqueue",
			nTasks:  1,
			wantErr: "\"squeue\" command is not supported",
		},
		"unsupported squeue command with parameters": {
			script:  "#!/bin/bash\nsqueue -s -p debug -S u",
			nTasks:  1,
			wantErr: "\"squeue\" command is not supported",
		},
		"unsupported scancel command": {
			script:  "#!/bin/bash\nscancel",
			nTasks:  1,
			wantErr: "\"scancel\" command is not supported",
		},
		"unsupported scancel command with parameters": {
			script:  "#!/bin/bash\nscancel 1234",
			nTasks:  1,
			wantErr: "\"scancel\" command is not supported",
		},
		"valid multiline command": {
			script: "#!/bin/bash\nsrun \\\nmy_program",
			nTasks: 1,
			want:   "#!/bin/bash\nmy_program",
		},
		"command with env": {
			script: "#!/bin/bash\nENV1=test1; ENV2=\"test 2\"; export ENV3=\"test 3\"; srun -n 1 my_program",
			nTasks: 1,
			want:   "#!/bin/bash\nENV1=test1; ENV2=\"test 2\"; export ENV3=\"test 3\"; my_program",
		},
		"command with suffix on srun": {
			script: "#!/bin/bash\nssrun -n 1 my_program",
			nTasks: 1,
			want:   "#!/bin/bash\nssrun -n 1 my_program",
		},
		"command with suffix and env": {
			script: "#!/bin/bash\nENV=test ssrun -n 1 my_program",
			nTasks: 1,
			want:   "#!/bin/bash\nENV=test ssrun -n 1 my_program",
		},
		"command with srun prefix": {
			script: "#!/bin/bash\nsrunish -n 1 my_program",
			nTasks: 1,
			want:   "#!/bin/bash\nsrunish -n 1 my_program",
		},
		"command with srun prefix and env": {
			script: "#!/bin/bash\nENV=test srunish -n 1 my_program",
			nTasks: 1,
			want:   "#!/bin/bash\nENV=test srunish -n 1 my_program",
		},
		"env vars with invalid env variables": {
			script:  "#!/bin/bash\n\"ENV1=test1\" srun -n 1 my_program",
			nTasks:  1,
			wantErr: "invalid \"\\\"ENV1=test1\\\" srun -n 1 my_program\" command: invalid \"\\\"ENV1=test1\\\"\" environment variable",
		},
		"srun with double quotes": {
			script: "srun -flag1 \"arguments for flag 1\" my_program",
			nTasks: 1,
			want:   "my_program",
		},
		"srun with single quotes": {
			script: "srun -flag1 'arguments for flag 1' my_program",
			nTasks: 1,
			want:   "my_program",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := SlurmValidateAndReplaceScript(tc.script, tc.nTasks)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected script (-want,+got):\n%s", diff)
			}

			var gotErrStr string
			if gotErr != nil {
				gotErrStr = gotErr.Error()
			}

			if diff := cmp.Diff(tc.wantErr, gotErrStr, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestValidateEnvVars(t *testing.T) {
	testCases := map[string]struct {
		str     string
		wantErr string
	}{
		"invalid export not closed": {
			str:     "FOO=foo BAR=\"bar\" export BAZ=baz ",
			wantErr: "missed ';' or '&&' before \"test\" command",
		},
		"invalid export not closed first export": {
			str:     "FOO=foo export BAR='bar' export BAZ=baz; ",
			wantErr: "missed ';' or '&&' before \"export\" command",
		},
		"escape semicolon": {
			str:     "FOO=foo BAR=\"bar\" export BAZ=baz\\; ",
			wantErr: "missed ';' or '&&' before \"test\" command",
		},
		"double semicolon": {
			str:     "FOO=foo BAR=\"bar\" export BAZ=baz;; ",
			wantErr: "parse error near ';;'",
		},
		"valid with semicolon": {
			str: "FOO=foo BAR=\"bar\" export BAZ=baz; ",
		},
		"valid with and": {
			str: "FOO=foo BAR=\"bar\" export BAZ=baz && ",
		},
		"invalid env variables": {
			str:     "FOO-foo ",
			wantErr: "invalid \"FOO-foo\" environment variable",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateEnvVars(tc.str, "test")

			var gotErrStr string
			if gotErr != nil {
				gotErrStr = gotErr.Error()
			}

			if diff := cmp.Diff(tc.wantErr, gotErrStr); diff != "" {
				t.Errorf("Unexpected error (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestSplitBySpaceWithIgnoreInQuotes(t *testing.T) {
	testCases := map[string]struct {
		str  string
		want []string
	}{
		"without quotes": {
			str:  "a b c",
			want: []string{"a", "b", "c"},
		},
		"with double quotes": {
			str:  "a \"b c\"",
			want: []string{"a", "\"b c\""},
		},
		"with single quotes": {
			str:  "a 'b c'",
			want: []string{"a", "'b c'"},
		},
		"with single and double quotes": {
			str:  "a '\"b\" \"c\"' \"'d' 'e'\"",
			want: []string{"a", "'\"b\" \"c\"'", "\"'d' 'e'\""},
		},
		"with escaped double quotes": {
			str:  "a \\\"b c\\\"",
			want: []string{"a", "\\\"b", "c\\\""},
		},
		"with escaped single quotes": {
			str:  "a \\'b c\\'",
			want: []string{"a", "\\'b", "c\\'"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := splitBySpaceWithIgnoreInQuotes(tc.str)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestIsEscaped(t *testing.T) {
	testCases := map[string]struct {
		str   string
		index int
		want  bool
	}{
		"escaped": {
			str:   "\\\\;",
			index: 1,
			want:  true,
		},
		"not escaped": {
			str:   "\\\\;",
			index: 2,
			want:  false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := isEscaped(tc.str, tc.index)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}
		})
	}
}
