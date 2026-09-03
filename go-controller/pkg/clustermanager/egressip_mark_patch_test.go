// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package clustermanager

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/onsi/gomega"

	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/util"
)

// applyJSONPatch applies a minimal subset of RFC 6902 ("test" on
// "/metadata/resourceVersion", "add" on "/metadata/annotations" and
// "/metadata/annotations/<key>") to an annotations map, mirroring how the
// API server would apply the patch operations produced by
// generateMarkPatchOps. currentResourceVersion simulates the object's actual
// resourceVersion at apply time, which may differ from what the patch was
// generated against.
func applyJSONPatch(t *testing.T, annotations map[string]string, currentResourceVersion string, ops []jsonPatchOperation) (map[string]string, error) {
	t.Helper()
	result := map[string]string{}
	for k, v := range annotations {
		result[k] = v
	}
	for _, op := range ops {
		switch op.Operation {
		case "test":
			if op.Path != "/metadata/resourceVersion" {
				t.Fatalf("unexpected test path %q in test helper", op.Path)
			}
			if op.Value != currentResourceVersion {
				return nil, fmt.Errorf("test op failed: expected resourceVersion %q, got %q", op.Value, currentResourceVersion)
			}
		case "add":
			if op.Path == "/metadata/annotations" {
				result = map[string]string{}
				continue
			}
			raw, err := json.Marshal(op.Value)
			if err != nil {
				t.Fatalf("failed to marshal patch value: %v", err)
			}
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatalf("failed to unmarshal patch value: %v", err)
			}
			result[util.EgressIPMarkAnnotation] = value
		default:
			t.Fatalf("unexpected op %q in test helper", op.Operation)
		}
	}
	return result, nil
}

func TestGenerateMarkPatchOpsPreservesExistingAnnotations(t *testing.T) {
	g := gomega.NewWithT(t)

	existing := map[string]string{
		"argocd.argoproj.io/tracking-id": "openshift-gitops:k8s.ovn.org/EgressIP:openshift-gitops",
	}

	ops := generateMarkPatchOps(existing, "1", 50001)

	// Only the specific annotation leaf should be targeted, not the whole map,
	// and no resourceVersion guard is needed since annotations already exists.
	g.Expect(ops).To(gomega.HaveLen(1))
	g.Expect(ops[0].Path).To(gomega.Equal("/metadata/annotations/k8s.ovn.org~1egressip-mark"))

	result, err := applyJSONPatch(t, existing, "1", ops)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.HaveKeyWithValue("argocd.argoproj.io/tracking-id", "openshift-gitops:k8s.ovn.org/EgressIP:openshift-gitops"))
	g.Expect(result).To(gomega.HaveKeyWithValue(util.EgressIPMarkAnnotation, "50001"))
}

func TestGenerateMarkPatchOpsCreatesAnnotationsMapWhenMissing(t *testing.T) {
	g := gomega.NewWithT(t)

	ops := generateMarkPatchOps(nil, "1", 50002)

	g.Expect(ops).To(gomega.HaveLen(3))
	g.Expect(ops[0].Operation).To(gomega.Equal("test"))
	g.Expect(ops[0].Path).To(gomega.Equal("/metadata/resourceVersion"))
	g.Expect(ops[0].Value).To(gomega.Equal("1"))
	g.Expect(ops[1].Path).To(gomega.Equal("/metadata/annotations"))
	g.Expect(ops[2].Path).To(gomega.Equal("/metadata/annotations/k8s.ovn.org~1egressip-mark"))

	result, err := applyJSONPatch(t, nil, "1", ops)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.HaveKeyWithValue(util.EgressIPMarkAnnotation, "50002"))
}

func TestGenerateMarkPatchOpsFailsClosedOnConcurrentAnnotationAdd(t *testing.T) {
	g := gomega.NewWithT(t)

	// Patch generated while annotations was still empty at resourceVersion "1".
	ops := generateMarkPatchOps(nil, "1", 50003)

	// By the time the patch is applied, a concurrent writer has bumped the
	// resourceVersion by adding its own annotation. The whole patch must be
	// rejected rather than silently discarding that annotation.
	concurrentlyAdded := map[string]string{"argocd.argoproj.io/tracking-id": "some-app"}
	_, err := applyJSONPatch(t, concurrentlyAdded, "2", ops)
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestGenerateMarkPatchOpsSkipsGuardWhenResourceVersionUnknown(t *testing.T) {
	g := gomega.NewWithT(t)

	ops := generateMarkPatchOps(nil, "", 50004)

	g.Expect(ops).To(gomega.HaveLen(2))
	g.Expect(ops[0].Path).To(gomega.Equal("/metadata/annotations"))
	g.Expect(ops[1].Path).To(gomega.Equal("/metadata/annotations/k8s.ovn.org~1egressip-mark"))
}

func TestEscapeJSONPatchPathKey(t *testing.T) {
	g := gomega.NewWithT(t)

	g.Expect(escapeJSONPatchPathKey(util.EgressIPMarkAnnotation)).To(gomega.Equal("k8s.ovn.org~1egressip-mark"))
	g.Expect(escapeJSONPatchPathKey("a~b/c")).To(gomega.Equal("a~0b~1c"))
}
