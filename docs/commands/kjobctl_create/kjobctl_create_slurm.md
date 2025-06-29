<!--
The file is auto-generated from the Go source code of the component using the
[generator](https://github.com/kubernetes-sigs/kueue/tree/main/cmd/experimental/kjobctl/cmd/kjobctl-docs).
-->

# kjobctl create slurm


## Synopsis


Create a slurm job

```
kjobctl create slurm --profile APPLICATION_PROFILE_NAME [--localqueue LOCAL_QUEUE_NAME] [--skip-localqueue-validation] [--priority NAME] [--skip-priority-validation] [--ignore-unknown-flags] [--wait] [--wait-timeout] [--init-image IMAGE] [--first-node-ip] [--first-node-ip-timeout DURATION] [--rm] [--stream-container NAME] [--worker-container NAME] --  [--array ARRAY] [--cpus-per-task QUANTITY] [--gpus-per-task QUANTITY] [--mem QUANTITY] [--mem-per-task QUANTITY] [--mem-per-cpu QUANTITY] [--mem-per-gpu QUANTITY] [--nodes COUNT] [--ntasks COUNT] [--ntasks-per-node COUNT] [--output FILENAME_PATTERN] [--error FILENAME_PATTERN] [--input FILENAME_PATTERN] [--job-name NAME] [--partition NAME] SCRIPT
```


## Examples

```
  # Create slurm
  kjobctl create slurm --profile my-application-profile -- \
  --array 0-5 --nodes 3 --ntasks 1 ./script.sh
```


## Options


<table style="width: 100%; table-layout: fixed;">
    <colgroup>
        <col span="1" style="width: 10px;" />
        <col span="1" />
    </colgroup>
    <tbody>
    <tr>
        <td colspan="2">--allow-missing-template-keys&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: true</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--dry-run string&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: &#34;none&#34;</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Must be &#34;none&#34;, &#34;server&#34;, or &#34;client&#34;. If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--first-node-ip</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Enable the retrieval of the first node&#39;s IP address.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--first-node-ip-timeout duration&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: 1m0s</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The timeout for the retrieval of the first node&#39;s IP address.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">-h, --help</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>help for slurm</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--ignore-unknown-flags</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Ignore all the unsupported flags in the bash script.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--init-image string&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: &#34;registry.k8s.io/busybox:1.27.2&#34;</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The image used for the init container.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--localqueue string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Kueue localqueue name which is associated with the resource.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">-o, --output string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Output format. One of: (json, yaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--pod-template-annotation &lt;comma-separated &#39;key=value&#39; pairs&gt;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: []</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Specifies one or more annotations for the Pod template.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--pod-template-label &lt;comma-separated &#39;key=value&#39; pairs&gt;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: []</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Specifies one or more labels for the Pod template.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--priority string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Apply priority for the entire workload.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">-p, --profile string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Application profile contains a template (with defaults set) for running a specific type of application.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--rm</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Remove the job when it is canceled while running with the wait flag.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--show-managed-fields</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>If true, keep the managedFields when printing objects in JSON or YAML format.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--skip-localqueue-validation</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Skip local queue validation. Add local queue even if the queue does not exist.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--skip-priority-validation</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Skip workload priority class validation. Add priority class label even if the class does not exist.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--stream-container strings</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Specify containers from which logs will be streamed. This can be set only if --wait is used. By default, all containers are included.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--template string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--wait</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Wait for the Job to complete and stream its Pod logs.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--wait-timeout duration</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Timeout for waiting for the job to complete.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--worker-container strings</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Specify containers for which limits and volumes will be applied. By default, all containers are included.</p>
        </td>
    </tr>
    </tbody>
</table>



## Options inherited from parent commands
<table style="width: 100%; table-layout: fixed;">
    <colgroup>
        <col span="1" style="width: 10px;" />
        <col span="1" />
    </colgroup>
    <tbody>
    <tr>
        <td colspan="2">--as string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Username to impersonate for the operation. User could be a regular user or a service account in a namespace.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--as-group strings</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Group to impersonate for the operation, this flag can be repeated to specify multiple groups.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--as-uid string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>UID to impersonate for the operation.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--cache-dir string&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: &#34;$HOME/.kube/cache&#34;</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Default cache directory</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--certificate-authority string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Path to a cert file for the certificate authority</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--client-certificate string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Path to a client certificate file for TLS</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--client-key string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Path to a client key file for TLS</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--cluster string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The name of the kubeconfig cluster to use</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--context string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The name of the kubeconfig context to use</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--disable-compression</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>If true, opt-out of response compression for all requests to the server</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--insecure-skip-tls-verify</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>If true, the server&#39;s certificate will not be checked for validity. This will make your HTTPS connections insecure</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--kubeconfig string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Path to the kubeconfig file to use for CLI requests.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">-n, --namespace string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>If present, the namespace scope for this CLI request</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--request-timeout string&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Default: &#34;0&#34;</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don&#39;t timeout requests.</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">-s, --server string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The address and port of the Kubernetes API server</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--tls-server-name string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--token string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>Bearer token for authentication to the API server</p>
        </td>
    </tr>
    <tr>
        <td colspan="2">--user string</td>
    </tr>
    <tr>
        <td></td>
        <td style="line-height: 130%; word-wrap: break-word;">
            <p>The name of the kubeconfig user to use</p>
        </td>
    </tr>
    </tbody>
</table>



## See Also

* [kjobctl_create](_index.md)	 - Create a task

