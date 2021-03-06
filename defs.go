//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package cbft

import (
	"encoding/json"
	"reflect"

	"github.com/couchbaselabs/blance"
)

// JSON/struct definitions of what the Manager stores in the Cfg.
// NOTE: You *must* update VERSION if you change these
// definitions or the planning algorithms change.

type IndexDefs struct {
	// IndexDefs.UUID changes whenever any child IndexDef changes.
	UUID        string               `json:"uuid"`
	IndexDefs   map[string]*IndexDef `json:"indexDefs"`   // Key is IndexDef.Name.
	ImplVersion string               `json:"implVersion"` // See VERSION.
}

type IndexDef struct {
	Type         string     `json:"type"` // Ex: "bleve", "alias", "blackhole", etc.
	Name         string     `json:"name"`
	UUID         string     `json:"uuid"`
	Params       string     `json:"params"`
	SourceType   string     `json:"sourceType"`
	SourceName   string     `json:"sourceName"`
	SourceUUID   string     `json:"sourceUUID"`
	SourceParams string     `json:"sourceParams"` // Optional connection info.
	PlanParams   PlanParams `json:"planParams"`

	// NOTE: Any auth credentials to access datasource, if any, may be
	// stored as part of SourceParams.
}

type PlanParams struct {
	// MaxPartitionsPerPIndex controls the maximum number of source
	// partitions the planner can assign to or clump into a PIndex (or
	// index partition).
	MaxPartitionsPerPIndex int `json:"maxPartitionsPerPIndex"`

	// NumReplicas controls the number of replicas for a PIndex, over
	// the first copy.  The first copy is not counted as a replica.
	// For example, a NumReplicas setting of 2 means there should be a
	// primary and 2 replicas... so 3 copies in total.  A NumReplicas
	// of 0 means just the first, primary copy only.
	NumReplicas int `json:"numReplicas"`

	// HierarchyRules defines the policy the planner should follow
	// when assigning PIndexes to nodes, especially for replica
	// placement.  Through the HierarchyRules, a user can specify, for
	// example, that the first replica should be not on the same rack
	// and zone as the first copy.
	HierarchyRules blance.HierarchyRules `json:"hierarchyRules"`

	// NodePlanParams allows users to specify per-node input to the
	// planner, such as whether PIndexes assigned to different nodes
	// can be readable or writable.  Keyed by node UUID.  Value is
	// keyed by planPIndex.Name or indexDef.Name.  The empty string
	// ("") is used to represent any node UUID and/or any planPIndex
	// and/or any indexDef.
	NodePlanParams map[string]map[string]*NodePlanParam `json:"nodePlanParams"`

	// PlanFrozen means the planner should not change the previous
	// plan for an index, even if as nodes join or leave and even if
	// there was no previous plan.  Defaults to false (allow
	// re-planning).
	PlanFrozen bool `json:"planFrozen"`
}

type NodePlanParam struct {
	CanRead  bool `json:"canRead"`
	CanWrite bool `json:"canWrite"`
}

// ------------------------------------------------------------------------

type NodeDefs struct {
	// NodeDefs.UUID changes whenever any child NodeDef changes.
	UUID        string              `json:"uuid"`
	NodeDefs    map[string]*NodeDef `json:"nodeDefs"`    // Key is NodeDef.HostPort.
	ImplVersion string              `json:"implVersion"` // See VERSION.
}

type NodeDef struct {
	HostPort    string   `json:"hostPort"`
	UUID        string   `json:"uuid"`
	ImplVersion string   `json:"implVersion"` // See VERSION.
	Tags        []string `json:"tags"`
	Container   string   `json:"container"`
	Weight      int      `json:"weight"`
}

// ------------------------------------------------------------------------

type PlanPIndexes struct {
	// PlanPIndexes.UUID changes whenever any child PlanPIndex changes.
	UUID         string                 `json:"uuid"`
	PlanPIndexes map[string]*PlanPIndex `json:"planPIndexes"` // Key is PlanPIndex.Name.
	ImplVersion  string                 `json:"implVersion"`  // See VERSION.
	Warnings     map[string][]string    `json:"warnings"`     // Key is IndexDef.Name.
}

type PlanPIndex struct {
	Name             string `json:"name"` // Stable & unique cluster wide.
	UUID             string `json:"uuid"`
	IndexType        string `json:"indexType"`   // See IndexDef.Type.
	IndexName        string `json:"indexName"`   // See IndexDef.Name.
	IndexUUID        string `json:"indexUUID"`   // See IndefDef.UUID.
	IndexParams      string `json:"indexParams"` // See IndexDef.Params.
	SourceType       string `json:"sourceType"`
	SourceName       string `json:"sourceName"`
	SourceUUID       string `json:"sourceUUID"`
	SourceParams     string `json:"sourceParams"` // Optional connection info.
	SourcePartitions string `json:"sourcePartitions"`

	Nodes map[string]*PlanPIndexNode `json:"nodes"` // Keyed by NodeDef.UUID.
}

type PlanPIndexNode struct {
	CanRead  bool `json:"canRead"`
	CanWrite bool `json:"canWrite"`
	Priority int  `json:"priority"`
}

func PlanPIndexNodeCanRead(p *PlanPIndexNode) bool {
	return p != nil && p.CanRead
}

func PlanPIndexNodeCanWrite(p *PlanPIndexNode) bool {
	return p != nil && p.CanWrite
}

// ------------------------------------------------------------------------

const INDEX_DEFS_KEY = "indexDefs"

func NewIndexDefs(version string) *IndexDefs {
	return &IndexDefs{
		UUID:        NewUUID(),
		IndexDefs:   make(map[string]*IndexDef),
		ImplVersion: version,
	}
}

func CfgGetIndexDefs(cfg Cfg) (*IndexDefs, uint64, error) {
	v, cas, err := cfg.Get(INDEX_DEFS_KEY, 0)
	if err != nil {
		return nil, 0, err
	}
	if v == nil {
		return nil, 0, nil
	}
	rv := &IndexDefs{}
	err = json.Unmarshal(v, rv)
	if err != nil {
		return nil, 0, err
	}
	return rv, cas, nil
}

func CfgSetIndexDefs(cfg Cfg, indexDefs *IndexDefs, cas uint64) (uint64, error) {
	buf, err := json.Marshal(indexDefs)
	if err != nil {
		return 0, err
	}
	return cfg.Set(INDEX_DEFS_KEY, buf, cas)
}

// ------------------------------------------------------------------------

func GetNodePlanParam(nodePlanParams map[string]map[string]*NodePlanParam,
	nodeUUID, indexDefName, planPIndexName string) *NodePlanParam {
	var nodePlanParam *NodePlanParam
	if nodePlanParams != nil {
		m := nodePlanParams[nodeUUID]
		if m == nil {
			m = nodePlanParams[""]
		}
		if m != nil {
			nodePlanParam = m[indexDefName]
			if nodePlanParam == nil {
				nodePlanParam = m[planPIndexName]
			}
			if nodePlanParam == nil {
				nodePlanParam = m[""]
			}
		}
	}
	return nodePlanParam
}

// ------------------------------------------------------------------------

const NODE_DEFS_KEY = "nodeDefs"
const NODE_DEFS_KNOWN = "known"
const NODE_DEFS_WANTED = "wanted"

func NewNodeDefs(version string) *NodeDefs {
	return &NodeDefs{
		UUID:        NewUUID(),
		NodeDefs:    make(map[string]*NodeDef),
		ImplVersion: version,
	}
}

func CfgNodeDefsKey(kind string) string {
	return NODE_DEFS_KEY + "-" + kind
}

func CfgGetNodeDefs(cfg Cfg, kind string) (*NodeDefs, uint64, error) {
	v, cas, err := cfg.Get(CfgNodeDefsKey(kind), 0)
	if err != nil {
		return nil, 0, err
	}
	if v == nil {
		return nil, 0, nil
	}
	rv := &NodeDefs{}
	err = json.Unmarshal(v, rv)
	if err != nil {
		return nil, 0, err
	}
	return rv, cas, nil
}

func CfgSetNodeDefs(cfg Cfg, kind string, nodeDefs *NodeDefs,
	cas uint64) (uint64, error) {
	buf, err := json.Marshal(nodeDefs)
	if err != nil {
		return 0, err
	}
	return cfg.Set(CfgNodeDefsKey(kind), buf, cas)
}

// ------------------------------------------------------------------------

const PLAN_PINDEXES_KEY = "planPIndexes"

func NewPlanPIndexes(version string) *PlanPIndexes {
	return &PlanPIndexes{
		UUID:         NewUUID(),
		PlanPIndexes: make(map[string]*PlanPIndex),
		ImplVersion:  version,
		Warnings:     make(map[string][]string),
	}
}

func CfgGetPlanPIndexes(cfg Cfg) (*PlanPIndexes, uint64, error) {
	v, cas, err := cfg.Get(PLAN_PINDEXES_KEY, 0)
	if err != nil {
		return nil, 0, err
	}
	if v == nil {
		return nil, 0, nil
	}
	rv := &PlanPIndexes{}
	err = json.Unmarshal(v, rv)
	if err != nil {
		return nil, 0, err
	}
	return rv, cas, nil
}

func CfgSetPlanPIndexes(cfg Cfg, planPIndexes *PlanPIndexes, cas uint64) (uint64, error) {
	buf, err := json.Marshal(planPIndexes)
	if err != nil {
		return 0, err
	}
	return cfg.Set(PLAN_PINDEXES_KEY, buf, cas)
}

// Returns true if both PlanPIndexes are the same, where we ignore any
// differences in UUID or ImplVersion.
func SamePlanPIndexes(a, b *PlanPIndexes) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a.PlanPIndexes) != len(b.PlanPIndexes) {
		return false
	}
	return SubsetPlanPIndexes(a, b) && SubsetPlanPIndexes(b, a)
}

// Returns true if PlanPIndex children in a are a subset of those in
// b, using SamePlanPIndex() for sameness comparion.
func SubsetPlanPIndexes(a, b *PlanPIndexes) bool {
	for name, av := range a.PlanPIndexes {
		bv, exists := b.PlanPIndexes[name]
		if !exists {
			return false
		}
		if !SamePlanPIndex(av, bv) {
			return false
		}
	}
	return true
}

// Returns true if both PlanPIndex are the same, ignoring PlanPIndex.UUID.
func SamePlanPIndex(a, b *PlanPIndex) bool {
	// Of note, we don't compare UUID's.
	if a.Name != b.Name ||
		a.IndexName != b.IndexName ||
		a.IndexUUID != b.IndexUUID ||
		a.IndexParams != b.IndexParams ||
		a.SourceType != b.SourceType ||
		a.SourceName != b.SourceName ||
		a.SourceUUID != b.SourceUUID ||
		a.SourceParams != b.SourceParams ||
		a.SourcePartitions != b.SourcePartitions ||
		!reflect.DeepEqual(a.Nodes, b.Nodes) {
		return false
	}
	return true
}

// Returns true if both the PIndex meets the PlanPIndex, ignoring UUID.
func PIndexMatchesPlan(pindex *PIndex, planPIndex *PlanPIndex) bool {
	same := pindex.Name == planPIndex.Name &&
		pindex.IndexName == planPIndex.IndexName &&
		pindex.IndexUUID == planPIndex.IndexUUID &&
		pindex.IndexParams == planPIndex.IndexParams &&
		pindex.SourceType == planPIndex.SourceType &&
		pindex.SourceName == planPIndex.SourceName &&
		pindex.SourceUUID == planPIndex.SourceUUID &&
		pindex.SourceParams == planPIndex.SourceParams &&
		pindex.SourcePartitions == planPIndex.SourcePartitions
	return same
}
