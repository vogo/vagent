/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package orchestrate

// topologicalSort returns node IDs in topological order using Kahn's algorithm.
func topologicalSort(nodes []Node) []string {
	downstream := make(map[string][]string)
	inDeg := make(map[string]int)
	for _, n := range nodes {
		inDeg[n.ID] = len(n.Deps)
		for _, dep := range n.Deps {
			downstream[dep] = append(downstream[dep], n.ID)
		}
	}

	var order []string
	var queue []string
	for _, n := range nodes {
		if inDeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, ds := range downstream[cur] {
			inDeg[ds]--
			if inDeg[ds] == 0 {
				queue = append(queue, ds)
			}
		}
	}
	return order
}

// buildDAGMaps builds the downstream adjacency list and in-degree map from nodes.
func buildDAGMaps(nodes []Node) (downstream map[string][]string, inDegree map[string]int) {
	downstream = make(map[string][]string)
	inDegree = make(map[string]int)
	for _, n := range nodes {
		inDegree[n.ID] = len(n.Deps)
		for _, dep := range n.Deps {
			downstream[dep] = append(downstream[dep], n.ID)
		}
	}
	return
}
