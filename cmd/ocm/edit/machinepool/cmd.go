/*
Copyright (c) 2020 Red Hat, Inc.
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

package machinepool

import (
	"fmt"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/arguments"
	c "github.com/openshift-online/ocm-cli/pkg/cluster"
	"github.com/openshift-online/ocm-cli/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"
)

var args struct {
	clusterKey string
	replicas   int
	labels     string
	taints     string
}

var Cmd = &cobra.Command{
	Use:     "machinepool --cluster={NAME|ID|EXTERNAL_ID} [flags] MACHINE_POOL_ID",
	Aliases: []string{"machine-pool"},
	Short:   "Edit a cluster machine pool",
	Long:    "Edit a machine pool size.",
	Example: `  #  Update the number of replicas for machine pool with ID 'a1b2'
  ocm edit machinepool --replicas=3 --cluster=mycluster a1b2`,
	RunE: run,
}

func init() {
	flags := Cmd.Flags()

	flags.StringVarP(
		&args.clusterKey,
		"cluster",
		"c",
		"",
		"Name or ID or external_id of the cluster to edit the machine pool (required).",
	)
	//nolint:gosec
	Cmd.MarkFlagRequired("cluster")

	flags.IntVar(
		&args.replicas,
		"replicas",
		-1,
		"Restrict application route to direct, private connectivity.",
	)

	flags.StringVar(
		&args.labels,
		"labels",
		"",
		"Labels for machine pool. Format should be a comma-separated list of 'key=value'. "+
			"This list will overwrite any modifications made to Node labels on an ongoing basis.",
	)

	flags.StringVar(
		&args.taints,
		"taints",
		"",
		"Taints for machine pool. Format should be a comma-separated list of 'key=value:scheduleType'. "+
			"This list will overwrite any modifications made to Node taints on an ongoing basis.",
	)
}

func run(cmd *cobra.Command, argv []string) error {

	if len(argv) != 1 {
		return fmt.Errorf(
			"Expected exactly one command line parameter containing the machine pool ID")
	}

	machinePoolID := argv[0]

	// Check that the cluster key (name, identifier or external identifier) given by the user
	// is reasonably safe so that there is no risk of SQL injection:
	clusterKey := args.clusterKey
	if !c.IsValidClusterKey(clusterKey) {
		return fmt.Errorf(
			"Cluster name, identifier or external identifier '%s' isn't valid: it "+
				"must contain only letters, digits, dashes and underscores",
			clusterKey,
		)
	}

	// Create the client for the OCM API:
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		return fmt.Errorf("Failed to create OCM connection: %v", err)
	}
	defer connection.Close()

	// Get the client for the cluster management api
	clusterCollection := connection.ClustersMgmt().V1().Clusters()

	cluster, err := c.GetCluster(clusterCollection, clusterKey)
	if err != nil {
		return fmt.Errorf("Failed to get cluster '%s': %v", clusterKey, err)
	}

	labels := make(map[string]string)
	if args.labels != "" {
		for _, label := range strings.Split(args.labels, ",") {
			if !strings.Contains(label, "=") {
				return fmt.Errorf("Expected key=value format for label-match")
			}
			tokens := strings.Split(label, "=")
			labels[strings.TrimSpace(tokens[0])] = strings.TrimSpace(tokens[1])
		}
	}

	taintBuilders := []*cmv1.TaintBuilder{}

	if args.taints != "" {
		for _, taint := range strings.Split(args.taints, ",") {
			if !strings.Contains(taint, "=") || !strings.Contains(taint, ":") {
				return fmt.Errorf("Expected key=value:scheduleType format for taints")
			}
			tokens := strings.FieldsFunc(taint, arguments.Split)
			taintBuilders = append(taintBuilders, cmv1.NewTaint().Key(tokens[0]).Value(tokens[1]).Effect(tokens[2]))
		}
	}

	machinePoolBuilder := cmv1.NewMachinePool().ID(machinePoolID)

	if cmd.Flags().Changed("labels") {
		machinePoolBuilder = machinePoolBuilder.Labels(labels)
	}

	if cmd.Flags().Changed("taints") {
		machinePoolBuilder = machinePoolBuilder.Taints(taintBuilders...)
	}

	if cmd.Flags().Changed("replicas") {
		machinePoolBuilder = machinePoolBuilder.Replicas(args.replicas)
	}

	machinePool, err := machinePoolBuilder.Build()

	if err != nil {
		return fmt.Errorf("Failed to create machine pool body for cluster '%s': %v", clusterKey, err)
	}

	_, err = clusterCollection.
		Cluster(cluster.ID()).
		MachinePools().
		MachinePool(machinePoolID).
		Update().
		Body(machinePool).
		Send()
	if err != nil {
		return fmt.Errorf("Failed to edit machine pool for cluster '%s': %v", clusterKey, err)
	}
	return nil
}
