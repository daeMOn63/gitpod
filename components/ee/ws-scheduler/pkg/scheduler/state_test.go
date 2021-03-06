// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the Gitpod Enterprise Source Code License,
// See License.enterprise.txt in the project root folder.

package scheduler_test

import (
	"sort"
	"testing"

	sched "github.com/gitpod-io/gitpod/ws-scheduler/pkg/scheduler"

	corev1 "k8s.io/api/core/v1"
	res "k8s.io/apimachinery/pkg/api/resource"
)

func TestState(t *testing.T) {
	defaultNodeSet := func() []*corev1.Node {
		return []*corev1.Node{
			createNode("node1", "10000Mi", "0Mi", false, 100),
			createNode("node2", "10000Mi", "0Mi", false, 100),
			createNode("node3", "10000Mi", "0Mi", true, 100),
		}
	}

	tests := []struct {
		Desc            string
		RAMSafetyBuffer string
		Nodes           []*corev1.Node
		Pods            []*corev1.Pod
		Bindings        []*sched.Binding
		GhostsInvisible bool
		Expectation     string
	}{
		{
			Desc:            "no pods",
			RAMSafetyBuffer: "512Mi",
			Nodes:           defaultNodeSet(),
			Expectation: `- node1:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node2:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi`,
		},
		{
			Desc:            "other pods only",
			RAMSafetyBuffer: "512Mi",
			Nodes:           defaultNodeSet(),
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "0Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "0Mi", "node2", 10),
			},
			Expectation: `- node1:
  RAM: used 0+0+1573 of 9949, avail 8377 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node2:
  RAM: used 0+0+1049 of 9949, avail 8901 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi`,
		},
		{
			Desc:            "some headless pods",
			RAMSafetyBuffer: "512Mi",
			Nodes:           defaultNodeSet(),
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "0Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "0Mi", "node2", 10),
				createHeadlessWorkspacePod("hp1", "1000Mi", "0Mi", "node2", 10),
				createHeadlessWorkspacePod("hp2", "2220Mi", "0Mi", "node2", 10),
			},
			Expectation: `- node1:
  RAM: used 0+0+1573 of 9949, avail 8377 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node2:
  RAM: used 0+3377+1049 of 9949, avail 5524 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi`,
		},
		{
			Desc:            "some regular pods",
			RAMSafetyBuffer: "512Mi",
			Nodes:           defaultNodeSet(),
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "0Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "0Mi", "node2", 10),
				createWorkspacePod("hp1", "1000Mi", "0Mi", "node1", 10),
				createWorkspacePod("hp2", "3440Mi", "0Mi", "node1", 10),
			},
			Expectation: `- node1:
  RAM: used 4656+0+1573 of 9949, avail 3721 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node2:
  RAM: used 0+0+1049 of 9949, avail 8901 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 0, avail 0 Mi`,
		},
		{
			Desc:            "some regular pods with ",
			RAMSafetyBuffer: "512Mi",
			Nodes: []*corev1.Node{
				createNode("node1", "10000Mi", "20000Mi", false, 100),
				createNode("node2", "10000Mi", "10000Mi", false, 100),
				createNode("node3", "10000Mi", "10000Mi", true, 100),
			},
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "5000Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "2000Mi", "node2", 10),
				createWorkspacePod("hp1", "1000Mi", "5000Mi", "node1", 10),
				createWorkspacePod("hp2", "3440Mi", "5000Mi", "node1", 10),
			},
			Expectation: `- node1:
  RAM: used 4656+0+1573 of 9949, avail 3721 Mi
  Eph. Storage: used 10486+0+5243 of 20972, avail 5243 Mi
- node2:
  RAM: used 0+0+1049 of 9949, avail 8901 Mi
  Eph. Storage: used 0+0+2098 of 10486, avail 8389 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 10486, avail 10486 Mi`,
		},
		{
			Desc:            "bound but not listed",
			RAMSafetyBuffer: "512Mi",
			Nodes: []*corev1.Node{
				createNode("node1", "10000Mi", "20000Mi", false, 100),
			},
			Bindings: []*sched.Binding{
				{
					Pod:      createWorkspacePod("hp1", "1000Mi", "5000Mi", "node1", 10),
					NodeName: "node1",
				},
			},
			Expectation: `- node1:
  RAM: used 1049+0+0 of 9949, avail 8901 Mi
  Eph. Storage: used 5243+0+0 of 20972, avail 15729 Mi`,
		},
		{
			Desc:            "some pods and ghosts on nodes (ghosts VISIBLE)",
			RAMSafetyBuffer: "512Mi",
			Nodes: []*corev1.Node{
				createNode("node1", "10000Mi", "20000Mi", false, 100),
				createNode("node2", "10000Mi", "10000Mi", false, 100),
				createNode("node3", "10000Mi", "10000Mi", true, 100),
			},
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "500Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "200Mi", "node2", 10),
				createWorkspacePod("hp1", "1000Mi", "100Mi", "node1", 10),
				createWorkspacePod("hp2", "3440Mi", "100Mi", "node1", 10),
				createGhostPod("ghost1", "1500Mi", "100Mi", "node1", 10),
				createGhostPod("ghost2", "1500Mi", "100Mi", "node2", 10),
			},
			GhostsInvisible: false,
			Expectation: `- node1:
  RAM: used 6229+0+1573 of 9949, avail 2148 Mi
  Eph. Storage: used 315+0+525 of 20972, avail 20133 Mi
- node2:
  RAM: used 1573+0+1049 of 9949, avail 7328 Mi
  Eph. Storage: used 105+0+210 of 10486, avail 10172 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 10486, avail 10486 Mi`,
		},
		{
			Desc:            "some pods and ghosts on nodes (ghosts INVISIBLE)",
			RAMSafetyBuffer: "512Mi",
			Nodes: []*corev1.Node{
				createNode("node1", "10000Mi", "20000Mi", false, 100),
				createNode("node2", "10000Mi", "10000Mi", false, 100),
				createNode("node3", "10000Mi", "10000Mi", true, 100),
			},
			Pods: []*corev1.Pod{
				createNonWorkspacePod("existingPod1", "1500Mi", "500Mi", "node1", 10),
				createNonWorkspacePod("existingPod2", "1000Mi", "200Mi", "node2", 10),
				createWorkspacePod("hp1", "1000Mi", "100Mi", "node1", 10),
				createWorkspacePod("hp2", "3440Mi", "100Mi", "node1", 10),
				createGhostPod("ghost1", "1500Mi", "100Mi", "node1", 10),
				createGhostPod("ghost2", "1500Mi", "100Mi", "node2", 10),
			},
			GhostsInvisible: true,
			Expectation: `- node1:
  RAM: used 4656+0+1573 of 9949, avail 3721 Mi
  Eph. Storage: used 210+0+525 of 20972, avail 20238 Mi
- node2:
  RAM: used 0+0+1049 of 9949, avail 8901 Mi
  Eph. Storage: used 0+0+210 of 10486, avail 10277 Mi
- node3:
  RAM: used 0+0+0 of 9949, avail 9949 Mi
  Eph. Storage: used 0+0+0 of 10486, avail 10486 Mi`,
		},
	}

	for _, test := range tests {
		t.Run(test.Desc, func(t *testing.T) {
			ramSafetyBuffer := res.MustParse(test.RAMSafetyBuffer)
			state := sched.ComputeState(test.Nodes, test.Pods, test.Bindings, &ramSafetyBuffer, test.GhostsInvisible)

			nodes := state.SortNodesByAvailableRAM(sched.SortAsc)
			// in some tests the RAM sort order is not stable as nodes have the same amount of RAM.
			// This would intermittently break tests. We instead sort by name.
			sort.Slice(nodes, func(i, j int) bool { return nodes[i].Node.Name < nodes[j].Node.Name })

			actual := sched.DebugStringNodes(nodes...)
			if test.Expectation != actual {
				t.Errorf("expected debug string to be:\n%s, was:\n%s", test.Expectation, actual)
				return
			}
		})
	}
}
