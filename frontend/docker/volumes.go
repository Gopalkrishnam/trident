package docker

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	hash "github.com/mitchellh/hashstructure"
	dvp_utils "github.com/netapp/netappdvp/utils"

	"github.com/netapp/trident/config"
	"github.com/netapp/trident/core"
	"github.com/netapp/trident/storage"
	"github.com/netapp/trident/storage_attribute"
	"github.com/netapp/trident/storage_class"
)

// getStorageClass accepts a list of volume creation options and returns a
// matching storage class.  If the orchestrator already has a matching
// storage class, that is returned; otherwise a new one is created and
// registered with the orchestrator.
func getStorageClass(options map[string]string, o core.Orchestrator) (*storage_class.Config, error) {

	// Create a storage class based on available options
	newScConfig, err := makeStorageClass(options, o)
	if err != nil {
		return nil, err
	}

	// Check existing storage classes for a match based on the name
	sc := o.GetStorageClass(newScConfig.Name)
	if sc != nil {
		log.WithField("storageClass", sc.Config.Name).Debug("Matched existing storage class.")
		return sc.Config, nil
	}

	// No match found, so register the new storage class
	addedSc, err := o.AddStorageClass(newScConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"storageClass": addedSc.Config.Name,
		}).Error("Docker frontend couldn't add the storage class: ", err)
		return nil, err
	}

	return addedSc.Config, nil
}

// makeStorageClass accepts a list of volume creation options and creates a
// matching storage class.  The name of the new storage class contains a hash
// of the attributes it contains, thereby enabling comparison of storage
// classes generated by this method by simply comparing their names.
func makeStorageClass(options map[string]string, o core.Orchestrator) (*storage_class.Config, error) {

	scConfig := new(storage_class.Config)

	// Map aggregate/pool option to storagePools
	requiredPool := dvp_utils.GetV(options, "aggregate|pool", "")
	if requiredPool != "" {
	aggrLoop:
		for _, backend := range o.ListBackends() {
			for poolName := range backend.Storage {
				if poolName == requiredPool {
					scConfig.Pools = make(map[string][]string)
					scConfig.Pools[backend.Name] = []string{poolName}
					break aggrLoop
				}
			}
		}
	}

	// Map remaining options to storage class attributes
	scConfig.Attributes = make(map[string]storage_attribute.Request)
	for k, v := range options {
		// format: attribute: "type:value"
		req, err := storage_attribute.CreateAttributeRequestFromAttributeValue(k, v)
		if err != nil {
			log.WithFields(log.Fields{
				"storageClass":            scConfig.Name,
				"storageClass_parameters": options,
			}).Debug("Docker frontend ignoring storage class attribute: ", err)
			continue
		}
		scConfig.Attributes[k] = req
	}

	// Set name based on hash value
	scHash, err := hash.Hash(scConfig, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"storageClass":            scConfig.Name,
			"storageClass_parameters": options,
		}).Error("Docker frontend couldn't hash the storage class attributes: ", err)
		return nil, err
	}
	scConfig.Name = fmt.Sprintf(auto_storage_class_prefix, scHash)

	return scConfig, nil
}

// getVolumeConfig accepts a set of parameters describing a volume creation request
// and returns a volume config structure suitable for passing to the orchestrator core.
func getVolumeConfig(name, storageClass string, opts map[string]string) (*storage.VolumeConfig, error) {

	sizeBytes, err := dvp_utils.GetVolumeSizeBytes(opts, default_volume_size)
	if err != nil {
		return nil, fmt.Errorf("Error creating volume: %v", err)
	}
	delete(opts, "size")

	return &storage.VolumeConfig{
		Name:                name,
		Size:                fmt.Sprintf("%d", sizeBytes),
		StorageClass:        storageClass,
		Protocol:            config.ProtocolAny,
		AccessMode:          config.ModeAny,
		SpaceReserve:        dvp_utils.GetV(opts, "spaceReserve", ""),
		SecurityStyle:       dvp_utils.GetV(opts, "securityStyle", ""),
		SplitOnClone:        dvp_utils.GetV(opts, "splitOnClone", ""),
		SnapshotPolicy:      dvp_utils.GetV(opts, "snapshotPolicy", ""),
		ExportPolicy:        dvp_utils.GetV(opts, "exportPolicy", ""),
		SnapshotDir:         dvp_utils.GetV(opts, "snapshotDir", ""),
		UnixPermissions:     dvp_utils.GetV(opts, "unixPermissions", ""),
		BlockSize:           dvp_utils.GetV(opts, "blocksize", ""),
		FileSystem:          dvp_utils.GetV(opts, "fstype|fileSystemType", ""),
		Encryption:          dvp_utils.GetV(opts, "encryption", ""),
		CloneSourceVolume:   dvp_utils.GetV(opts, "from", ""),
		CloneSourceSnapshot: dvp_utils.GetV(opts, "fromSnapshot", ""),
	}, nil
}
