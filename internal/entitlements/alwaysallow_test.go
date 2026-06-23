package entitlements

import "testing"

func TestAlwaysAllowEnablesEverything(t *testing.T) {
	svc := NewAlwaysAllow()

	for _, f := range []Feature{
		FeatureBuckets, FeatureAccessKeys, FeatureObjectBrowser, FeatureUsage,
		FeatureAudit, FeatureAPITokens, FeatureMultiNode, FeatureSSO, FeatureRBACEnforced,
	} {
		if d := svc.Allowed(f, "default"); !d.Allowed {
			t.Errorf("OSS build must allow feature %q, got denied: %s", f, d.Reason)
		}
	}

	for _, res := range []Resource{ResourceTenants, ResourceProjects, ResourceBuckets, ResourceAccessKeys} {
		if lim := svc.Limit(res, "default"); !lim.Unlimited {
			t.Errorf("OSS build must be unlimited for %q, got Max=%d", res, lim.Max)
		}
	}
}
