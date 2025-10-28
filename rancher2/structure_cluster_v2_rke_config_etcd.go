package rancher2

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

// Flatteners

func flattenClusterV2RKEConfigETCDSnapshotS3(in *rkev1.ETCDSnapshotS3) []interface{} {
	if in == nil {
		return nil
	}

	obj := make(map[string]interface{})

	if in.Bucket != "" {
		obj["bucket"] = in.Bucket
	}
	if in.CloudCredentialName != "" {
		obj["cloud_credential_name"] = in.CloudCredentialName
	}
	if in.Endpoint != "" {
		obj["endpoint"] = in.Endpoint
	}
	if in.EndpointCA != "" {
		obj["endpoint_ca"] = in.EndpointCA
	}
	if in.Folder != "" {
		obj["folder"] = in.Folder
	}
	if in.Region != "" {
		obj["region"] = in.Region
	}
	obj["skip_ssl_verify"] = in.SkipSSLVerify

	return []interface{}{obj}
}

func flattenClusterV2RKEConfigETCD(in *rkev1.ETCD) []interface{} {
	if in == nil {
		return nil
	}

	obj := make(map[string]interface{})

	obj["disable_snapshots"] = in.DisableSnapshots

	if len(in.SnapshotScheduleCron) > 0 {
		obj["snapshot_schedule_cron"] = in.SnapshotScheduleCron
	}
	if in.SnapshotRetention > 0 {
		obj["snapshot_retention"] = in.SnapshotRetention
	}
	if in.S3 != nil {
		obj["s3_config"] = flattenClusterV2RKEConfigETCDSnapshotS3(in.S3)
	}

	return []interface{}{obj}
}

// Expanders

func expandClusterV2RKEConfigETCDSnapshotS3(p []interface{}) *rkev1.ETCDSnapshotS3 {
	if len(p) == 0 || p[0] == nil {
		return nil
	}

	obj := &rkev1.ETCDSnapshotS3{}

	in := p[0].(map[string]interface{})

	if v, ok := in["bucket"].(string); ok && v != "" {
		obj.Bucket = v
	}
	if v, ok := in["cloud_credential_name"].(string); ok && v != "" {
		obj.CloudCredentialName = v
	}
	if v, ok := in["endpoint"].(string); ok && v != "" {
		obj.Endpoint = v
	}
	if v, ok := in["endpoint_ca"].(string); ok && v != "" {
		obj.EndpointCA = v
	}
	if v, ok := in["folder"].(string); ok && v != "" {
		obj.Folder = v
	}
	if v, ok := in["region"].(string); ok && v != "" {
		obj.Region = v
	}
	obj.SkipSSLVerify = in["skip_ssl_verify"].(bool)

	return obj
}

func expandClusterV2RKEConfigETCD(p []interface{}) *rkev1.ETCD {
	if len(p) == 0 || p[0] == nil {
		return nil
	}

	obj := &rkev1.ETCD{}

	in := p[0].(map[string]interface{})

	obj.DisableSnapshots = in["disable_snapshots"].(bool)

	if v, ok := in["snapshot_schedule_cron"].(string); ok && v != "" {
		obj.SnapshotScheduleCron = v
	}
	if v, ok := in["snapshot_retention"].(int); ok && v > 0 {
		obj.SnapshotRetention = v
	}
	if v, ok := in["s3_config"].([]interface{}); ok && len(v) > 0 {
		obj.S3 = expandClusterV2RKEConfigETCDSnapshotS3(v)
	}

	return obj
}
