package db

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/juanfont/headscale/hscontrol/types"
	"gorm.io/gorm"
	"tailscale.com/types/ptr"
)

var (
	ErrPreAuthKeyNotFound          = errors.New("AuthKey not found")
	ErrPreAuthKeyExpired           = errors.New("AuthKey expired")
	ErrSingleUseAuthKeyHasBeenUsed = errors.New("AuthKey has already been used")
	ErrUserMismatch                = errors.New("user mismatch")
	ErrPreAuthKeyACLTagInvalid     = errors.New("AuthKey tag is invalid")
)

func (hsdb *HSDatabase) CreatePreAuthKey(
	userName string,
	reusable bool,
	ephemeral bool,
	expiration *time.Time,
	aclTags []string,
) (*types.PreAuthKey, error) {
	return Write(hsdb.DB, func(tx *gorm.DB) (*types.PreAuthKey, error) {
		return CreatePreAuthKey(tx, userName, reusable, ephemeral, expiration, aclTags)
	})
}

// CreatePreAuthKey creates a new PreAuthKey in a user, and returns it.
func CreatePreAuthKey(
	tx *gorm.DB,
	userName string,
	reusable bool,
	ephemeral bool,
	expiration *time.Time,
	aclTags []string,
) (*types.PreAuthKey, error) {
	user, err := GetUser(tx, userName)
	if err != nil {
		return nil, err
	}

	for _, tag := range aclTags {
		if !strings.HasPrefix(tag, "tag:") {
			return nil, fmt.Errorf(
				"%w: '%s' did not begin with 'tag:'",
				ErrPreAuthKeyACLTagInvalid,
				tag,
			)
		}
	}

	now := time.Now().UTC()
	kstr, err := generateKey()
	if err != nil {
		return nil, err
	}

	key := types.PreAuthKey{
		Key:        kstr,
		UserID:     user.ID,
		User:       *user,
		Reusable:   reusable,
		Ephemeral:  ephemeral,
		CreatedAt:  &now,
		Expiration: expiration,
	}

	if err := tx.Save(&key).Error; err != nil {
		return nil, fmt.Errorf("failed to create key in the database: %w", err)
	}

	if len(aclTags) > 0 {
		seenTags := map[string]bool{}

		for _, tag := range aclTags {
			if !seenTags[tag] {
				if err := tx.Save(&types.PreAuthKeyACLTag{PreAuthKeyID: key.ID, Tag: tag}).Error; err != nil {
					return nil, fmt.Errorf(
						"failed to create key tag in the database: %w",
						err,
					)
				}
				seenTags[tag] = true
			}
		}
	}

	return &key, nil
}

func (hsdb *HSDatabase) ListPreAuthKeys(userName string) ([]types.PreAuthKey, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) ([]types.PreAuthKey, error) {
		return ListPreAuthKeys(rx, userName)
	})
}

// ListPreAuthKeys returns the list of PreAuthKeys for a user.
func ListPreAuthKeys(tx *gorm.DB, userName string) ([]types.PreAuthKey, error) {
	user, err := GetUser(tx, userName)
	if err != nil {
		return nil, err
	}

	keys := []types.PreAuthKey{}
	if err := tx.Preload("User").Preload("ACLTags").Where(&types.PreAuthKey{UserID: user.ID}).Find(&keys).Error; err != nil {
		return nil, err
	}

	return keys, nil
}

func (hsdb *HSDatabase) GetPreAuthKey(
	user string,
	key string,
) (*types.PreAuthKey, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) (*types.PreAuthKey, error) {
		return GetPreAuthKey(rx, user, key)
	})
}

// GetPreAuthKey returns a PreAuthKey for a given key.
func GetPreAuthKey(tx *gorm.DB, user string, key string) (*types.PreAuthKey, error) {
	pak, err := ValidatePreAuthKey(tx, key)
	if err != nil {
		return nil, err
	}

	if pak.User.Name != user {
		return nil, ErrUserMismatch
	}

	return pak, nil
}

func (hsdb *HSDatabase) GetPreAuthKeysBySguUuids(
	sguUuids []string,
) ([]types.PreAuthKey, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) ([]types.PreAuthKey, error) {
		return GetPreAuthKeysBySguUuids(rx, sguUuids)
	})
}

// GetPreAuthKeysBySguUuids returns PreAuthKeys for a given sguUuid.
func GetPreAuthKeysBySguUuids(
	tx *gorm.DB,
	sguUuids []string,
) ([]types.PreAuthKey, error) {
	var tagSguUuids []string
	for _, sguUuid := range sguUuids {
		tag := fmt.Sprintf("tag:sguser_%v", sguUuid)
		tagSguUuids = append(tagSguUuids, tag)
	}

	preAuthKeys := []types.PreAuthKey{}
	if err := tx.Preload("User").Preload("ACLTags").
		Joins("JOIN pre_auth_key_acl_tags ON pre_auth_keys.id = pre_auth_key_acl_tags.pre_auth_key_id").
		Where("pre_auth_key_acl_tags.tag IN ?", tagSguUuids).
		Find(&preAuthKeys).Error; err != nil {
		return nil, err
	}

	return preAuthKeys, nil
}

func (hsdb *HSDatabase) GetPreAuthKeysByClientUuids(
	clientUuids []string,
) ([]types.PreAuthKey, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) ([]types.PreAuthKey, error) {
		return GetPreAuthKeysByClientUuids(rx, clientUuids)
	})
}

// GetPreAuthKeysByClientUuids returns PreAuthKeys for given clientUuids.
func GetPreAuthKeysByClientUuids(
	tx *gorm.DB,
	clientUuids []string,
) ([]types.PreAuthKey, error) {
	var tagClientUuids []string

	for _, clientUuid := range clientUuids {
		tag := fmt.Sprintf("tag:client_%v", clientUuid)
		tagClientUuids = append(tagClientUuids, tag)
	}

	preAuthKeys := []types.PreAuthKey{}
	if err := tx.Preload("User").Preload("ACLTags").
		Joins("JOIN pre_auth_key_acl_tags ON pre_auth_keys.id = pre_auth_key_acl_tags.pre_auth_key_id").
		Where("pre_auth_key_acl_tags.tag IN ?", tagClientUuids).
		Find(&preAuthKeys).Error; err != nil {
		return nil, err
	}

	return preAuthKeys, nil
}

func (hsdb *HSDatabase) DestroyPreAuthKey(pak types.PreAuthKey) error {
	return hsdb.Write(func(tx *gorm.DB) error {
		return DestroyPreAuthKey(tx, pak)
	})
}

// DestroyPreAuthKey destroys a preauthkey. Returns error if the PreAuthKey
// does not exist.
func DestroyPreAuthKey(tx *gorm.DB, pak types.PreAuthKey) error {
	return tx.Transaction(func(db *gorm.DB) error {
		if result := db.Unscoped().Where(types.PreAuthKeyACLTag{PreAuthKeyID: pak.ID}).Delete(&types.PreAuthKeyACLTag{}); result.Error != nil {
			return result.Error
		}

		if result := db.Unscoped().Delete(pak); result.Error != nil {
			return result.Error
		}

		return nil
	})
}

func (hsdb *HSDatabase) ExpirePreAuthKey(k *types.PreAuthKey) error {
	return hsdb.Write(func(tx *gorm.DB) error {
		return ExpirePreAuthKey(tx, k)
	})
}

// ExpirePreAuthKey marks a PreAuthKey as expired.
func ExpirePreAuthKey(tx *gorm.DB, k *types.PreAuthKey) error {
	if err := tx.Model(&k).Update("Expiration", time.Now()).Error; err != nil {
		return err
	}

	return nil
}

// UsePreAuthKey marks a PreAuthKey as used.
func UsePreAuthKey(tx *gorm.DB, k *types.PreAuthKey) error {
	k.Used = true
	if err := tx.Save(k).Error; err != nil {
		return fmt.Errorf("failed to update key used status in the database: %w", err)
	}

	return nil
}

func (hsdb *HSDatabase) DisablePreAuthKeys(k []types.PreAuthKey) error {
	return hsdb.Write(func(tx *gorm.DB) error {
		return DisablePreAuthKeys(tx, k)
	})
}

// DisablePreAuthKeys marks PreAuthKeys as used and non-reusable.
func DisablePreAuthKeys(tx *gorm.DB, k []types.PreAuthKey) error {
	for idx := range k {
		k[idx].Reusable = false
		k[idx].Used = true
	}

	if err := tx.Save(&k).Error; err != nil {
		return fmt.Errorf("failed to update keys used status in the database: %w", err)
	}

	return nil
}

func (hsdb *HSDatabase) EnablePreAuthKeys(k []types.PreAuthKey) error {
	return hsdb.Write(func(tx *gorm.DB) error {
		return EnablePreAuthKeys(tx, k)
	})
}

// EnablePreAuthKeys marks PreAuthKeys as reusable.
func EnablePreAuthKeys(tx *gorm.DB, k []types.PreAuthKey) error {
	for idx := range k {
		k[idx].Reusable = true
	}

	if err := tx.Save(&k).Error; err != nil {
		return fmt.Errorf("failed to update keys used status in the database: %w", err)
	}

	return nil
}

func (hsdb *HSDatabase) DestroyPreAuthKeys(k []types.PreAuthKey) error {
	return hsdb.Write(func(tx *gorm.DB) error {
		return DestroyPreAuthKeys(tx, k)
	})
}

// DestroyPreAuthKeys destroys preauthkeys. Returns error if the PreAuthKey
// does not exist.
func DestroyPreAuthKeys(tx *gorm.DB, k []types.PreAuthKey) error {
	for _, key := range k {
		if err := DestroyPreAuthKey(tx, key); err != nil {
			return err
		}
	}
	return nil
}

func (hsdb *HSDatabase) GetPreAuthKeyTagLock(tag string) (*PreAuthKeyTagLock, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) (*PreAuthKeyTagLock, error) {
		return GetPreAuthKeyTagLock(rx, tag)
	})
}

// GetPreAuthKeyTagLock returns a PreAuthKeyTagLock for a SGUser_tag.
func GetPreAuthKeyTagLock(tx *gorm.DB, tag string) (*PreAuthKeyTagLock, error) {
	var lockRecord PreAuthKeyTagLock

	if err := tx.Where("tag = ?", tag).First(&lockRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("PreAuthKeyTagLock not found for tag: %s", tag)
		}
		return nil, err
	}
	return &lockRecord, nil
}

// GetPreAuthKeyTagLockBySguUuid returns a PreAuthKeyTagLock for a sgu_uuid.
func (hsdb *HSDatabase) GetPreAuthKeyTagLockBySguUuid(
	sgu_uuid string,
) (*PreAuthKeyTagLock, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) (*PreAuthKeyTagLock, error) {
		return GetPreAuthKeyTagLockBySguUuid(rx, sgu_uuid)
	})
}

func GetPreAuthKeyTagLockBySguUuid(
	tx *gorm.DB,
	sgu_uuid string,
) (*PreAuthKeyTagLock, error) {
	sguserTag := fmt.Sprintf("tag:sguser_%s", sgu_uuid)
	return GetPreAuthKeyTagLock(tx, sguserTag)
}

// SetPreAuthKeyTagLock sets the lock status for a given SGUser UUID with optimized performance.
// Uses existing query functions to avoid code duplication.
func (hsdb *HSDatabase) SetPreAuthKeyTagLockBySguUuid(
	sgu_uuid string,
	isLock bool,
) error {
	_, err := Write(hsdb.DB, func(tx *gorm.DB) (bool, error) {
		return SetPreAuthKeyTagLockBySguUuid(tx, sgu_uuid, isLock)
	})
	return err
}

func SetPreAuthKeyTagLockBySguUuid(
	tx *gorm.DB,
	sgu_uuid string,
	isLock bool,
) (bool, error) {
	existingRecord, err := GetPreAuthKeyTagLockBySguUuid(tx, sgu_uuid)

	if err == nil {
		if existingRecord.IsLock != isLock {
			existingRecord.IsLock = isLock
			if err := tx.Model(&existingRecord).Where("tag = ?", existingRecord.Tag).Update("is_lock", isLock).Error; err != nil {
				return false, fmt.Errorf(
					"failed to update PreAuthKeyTagLock for UUID %s: %w",
					sgu_uuid,
					err,
				)
			}
		}
		return true, nil
	}

	if strings.Contains(err.Error(), "not found") {
		sguserTag := fmt.Sprintf("tag:sguser_%s", sgu_uuid)
		newRecord := PreAuthKeyTagLock{
			Tag:    sguserTag,
			IsLock: isLock,
		}
		if err := tx.Create(&newRecord).Error; err != nil {
			return false, fmt.Errorf(
				"failed to create PreAuthKeyTagLock for UUID %s: %w",
				sgu_uuid,
				err,
			)
		}
		return true, nil
	}

	return false, fmt.Errorf(
		"failed to query PreAuthKeyTagLock for UUID %s: %w",
		sgu_uuid,
		err,
	)
}

// CheckSGUserTagLock check SGUser_tag of preauthkeys. Returns error if the SGUser_tag is Locked.
func (hsdb *HSDatabase) CheckSGUserTagLock(pak *types.PreAuthKey) error {
	_, err := Read(hsdb.DB, func(rx *gorm.DB) (bool, error) {
		return CheckSGUserTagLock(rx, pak)
	})
	return err
}

func CheckSGUserTagLock(tx *gorm.DB, pak *types.PreAuthKey) (bool, error) {
	for _, aclTag := range pak.ACLTags {
		if strings.HasPrefix(aclTag.Tag, "tag:sguser") {
			lockRecord, err := GetPreAuthKeyTagLock(tx, aclTag.Tag)
			if err == nil && lockRecord.IsLock {
				return false, fmt.Errorf("SGUser tag %s is locked", aclTag.Tag)
			}
		}
	}
	return true, nil
}

func (hsdb *HSDatabase) ValidatePreAuthKey(k string) (*types.PreAuthKey, error) {
	return Read(hsdb.DB, func(rx *gorm.DB) (*types.PreAuthKey, error) {
		return ValidatePreAuthKey(rx, k)
	})
}

// ValidatePreAuthKey does the heavy lifting for validation of the PreAuthKey coming from a node
// If returns no error and a PreAuthKey, it can be used.
func ValidatePreAuthKey(tx *gorm.DB, k string) (*types.PreAuthKey, error) {
	pak := types.PreAuthKey{}
	if result := tx.Preload("User").Preload("ACLTags").First(&pak, "key = ?", k); errors.Is(
		result.Error,
		gorm.ErrRecordNotFound,
	) {
		return nil, ErrPreAuthKeyNotFound
	}

	if pak.Expiration != nil && pak.Expiration.Before(time.Now()) {
		return nil, ErrPreAuthKeyExpired
	}

	if pak.Reusable { // we don't need to check if has been used before
		return &pak, nil
	}

	nodes := types.Nodes{}
	if err := tx.
		Preload("AuthKey").
		Where(&types.Node{AuthKeyID: ptr.To(pak.ID)}).
		Find(&nodes).Error; err != nil {
		return nil, err
	}

	if len(nodes) != 0 || pak.Used {
		return nil, ErrSingleUseAuthKeyHasBeenUsed
	}

	return &pak, nil
}

func generateKey() (string, error) {
	size := 24
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
