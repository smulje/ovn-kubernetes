package status_manager

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	anpapi "sigs.k8s.io/network-policy-api/apis/v1alpha1"
	anpclientfake "sigs.k8s.io/network-policy-api/pkg/client/clientset/versioned/fake"
)

// TestCleanupStaleStatusConditions tests that stale status.conditions[] entries are removed
func TestCleanupStaleStatusConditions(t *testing.T) {
	// Create a fake ANP with stale conditions
	anp := &anpapi.AdminNetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-anp",
		},
		Status: anpapi.AdminNetworkPolicyStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "Ready-In-Zone-node1",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "SetupSucceeded",
					Message:            "Setting up OVN DB plumbing was successful",
				},
				{
					Type:               "Ready-In-Zone-node2",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "SetupSucceeded",
					Message:            "Setting up OVN DB plumbing was successful",
				},
				{
					Type:               "Ready-In-Zone-deleted-node",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "SetupSucceeded",
					Message:            "Setting up OVN DB plumbing was successful",
				},
				{
					Type:               "Ready-In-Zone-fake-deleted-node-xyz",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "SetupSucceeded",
					Message:            "Setting up OVN DB plumbing was successful",
				},
			},
		},
	}

	// Create fake client with the ANP
	fakeClient := anpclientfake.NewSimpleClientset(anp)

	// Create manager
	manager := &anpZoneDeleteCleanupManager{
		client: fakeClient,
	}

	// Current zones (node1 and node2 exist, deleted-node and fake-deleted-node-xyz do not)
	currentZones := sets.New("node1", "node2")

	// Get ANPs
	existingANPs, err := manager.GetANPs()
	if err != nil {
		t.Fatalf("Failed to get ANPs: %v", err)
	}

	// Get BANPs (none in this test)
	existingBANPs, err := manager.GetBANPs()
	if err != nil {
		t.Fatalf("Failed to get BANPs: %v", err)
	}

	// Run cleanup
	err = manager.cleanupStaleStatusConditions(currentZones, existingANPs, existingBANPs)
	if err != nil {
		t.Fatalf("cleanupStaleStatusConditions failed: %v", err)
	}

	// Get updated ANP
	updatedANP, err := fakeClient.PolicyV1alpha1().AdminNetworkPolicies().Get(context.TODO(), "test-anp", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated ANP: %v", err)
	}

	// Verify conditions
	expectedConditions := 2 // Only node1 and node2 should remain
	if len(updatedANP.Status.Conditions) != expectedConditions {
		t.Errorf("Expected %d conditions, got %d", expectedConditions, len(updatedANP.Status.Conditions))
	}

	// Verify the correct conditions remain
	conditionTypes := make(map[string]bool)
	for _, condition := range updatedANP.Status.Conditions {
		conditionTypes[condition.Type] = true
	}

	if !conditionTypes["Ready-In-Zone-node1"] {
		t.Error("Expected condition Ready-In-Zone-node1 to remain")
	}
	if !conditionTypes["Ready-In-Zone-node2"] {
		t.Error("Expected condition Ready-In-Zone-node2 to remain")
	}
	if conditionTypes["Ready-In-Zone-deleted-node"] {
		t.Error("Expected condition Ready-In-Zone-deleted-node to be removed")
	}
	if conditionTypes["Ready-In-Zone-fake-deleted-node-xyz"] {
		t.Error("Expected condition Ready-In-Zone-fake-deleted-node-xyz to be removed")
	}

	t.Logf("✓ Test passed: Stale conditions were successfully removed")
	t.Logf("  Remaining conditions: %v", conditionTypes)
}

// TestFindStaleZonesInStatus tests the helper function
func TestFindStaleZonesInStatus(t *testing.T) {
	manager := &anpZoneDeleteCleanupManager{}

	conditions := []metav1.Condition{
		{Type: "Ready-In-Zone-node1"},
		{Type: "Ready-In-Zone-node2"},
		{Type: "Ready-In-Zone-deleted-node"},
		{Type: "Ready-In-Zone-fake-deleted-node-xyz"},
	}

	currentZones := sets.New("node1", "node2")

	staleZones := manager.findStaleZonesInStatus(conditions, currentZones)

	expectedStale := []string{"deleted-node", "fake-deleted-node-xyz"}
	if len(staleZones) != len(expectedStale) {
		t.Errorf("Expected %d stale zones, got %d: %v", len(expectedStale), len(staleZones), staleZones)
	}

	staleSet := sets.New(staleZones...)
	for _, expected := range expectedStale {
		if !staleSet.Has(expected) {
			t.Errorf("Expected stale zone %s not found", expected)
		}
	}

	t.Logf("✓ Test passed: Found stale zones: %v", staleZones)
}
