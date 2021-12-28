package backup

import "context"

type (
	PCloudToken   string
	PCloudCluster struct {
		HasRegistered bool           `json:"has_registered"`
		Setting       *PCloudSetting `json:"setting,omitempty"`
	}
	PCloudSetting struct {
		CloudServiceProvider string `json:"cloud_service_provider"`
		SubscriptionPlan     string `json:"subscription_plan"`
		// Owner is the email or github ID who registered the cluster
		Owner string `json:"owner"`
	}

	BackupInfo struct {
		Name        string   `json:"name"`
		BackupTime  string   `json:"backup_time"`
		ArchiveSize uint64   `json:"archive_size"`
		TableFilter []string `json:"table_filter"`
	}
)

type PCloudService interface {
	GenToken(ctx context.Context) (PCloudToken, error)
	Auth(ctx context.Context, token PCloudToken) error
	Cluster(ctx context.Context) (PCloudCluster, error)
	SaveBackupMeta(ctx context.Context, backupInfo BackupInfo) error
}
