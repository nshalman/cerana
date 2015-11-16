package main

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/mistifyio/gozfs/nv"
)

const (
	DatasetFilesystem = "filesystem"
	DatasetSnapshot   = "snapshot"
	DatasetVolume     = "volume"
)

// Dataset is a ZFS dataset containing a simplified set of information
type Dataset struct {
	Name          string
	Origin        string
	Used          uint64
	Avail         uint64
	Mountpoint    string
	Compression   string
	Type          string
	Written       uint64
	Volsize       uint64
	Usedbydataset uint64
	Logicalused   uint64
	Quota         uint64
	ds            *ds
}

type ds struct {
	DMUObjsetStats *dmuObjsetStats `nv:"dmu_objset_stats"`
	Name           string          `nv:"name"`
	Properties     *dsProperties   `nv:"properties"`
}

type dmuObjsetStats struct {
	CreationTxg  uint64 `nv:"dds_creation_txg"`
	Guid         uint64 `nv:"dds_guid"`
	Inconsistent bool   `nv:"dds_inconsistent"`
	IsSnapshot   bool   `nv:"dds_is_snapshot"`
	NumClones    uint64 `nv:"dds_num_clonse"`
	Origin       string `nv:"dds_origin"`
	Type         string `nv:"dds_type"`
}

type dsProperties struct {
	Available            propUint64           `nv:"available"`
	Clones               propClones           `nv:"clones"`
	Compression          propStringWithSource `nv:"compression"`
	CompressRatio        propUint64           `nv:"compressratio"`
	CreateTxg            propUint64           `nv:"createtxg"`
	Creation             propUint64           `nv:"creation"`
	DeferDestroy         propUint64           `nv:"defer_destroy"`
	Guid                 propUint64           `nv:"guid"`
	LogicalReferenced    propUint64           `nv:"logicalreferenced"`
	LogicalUsed          propUint64           `nv:"logicalused"`
	Mountpoint           propStringWithSource `nv:"mountpoint"`
	ObjsetID             propUint64           `nv:"objsetid"`
	Origin               propString           `nv:"origin"`
	Quota                propUint64WithSource `nv:"quota"`
	RefCompressRatio     propUint64           `nv:"refcompressratio"`
	RefQuota             propUint64WithSource `nv:"refquota"`
	RefReservation       propUint64WithSource `nv:"refreservation"`
	Referenced           propUint64           `nv:"referenced"`
	Reservation          propUint64WithSource `nv:"reservation"`
	Type                 propUint64           `nv:"type"`
	Unique               propUint64           `nv:"unique"`
	Used                 propUint64           `nv:"used"`
	UsedByChildren       propUint64           `nv:"usedbychildren"`
	UsedByDataset        propUint64           `nv:"usedbydataset"`
	UsedByRefReservation propUint64           `nv:"usedbyrefreservation"`
	UsedBySnapshots      propUint64           `nv:"usedbysnapshots"`
	UserAccounting       propUint64           `nv:"useraccounting"`
	UserRefs             propUint64           `nv:"userrefs"`
	Volsize              propUint64           `nv:"volsize"`
	VolBlockSize         propUint64           `nv:"volblocksize"`
	Written              propUint64           `nv:"written"`
}

var dsPropertyIndexes map[string]int

type dsProperty interface {
	value() interface{}
}

type propClones struct {
	Value map[string]nv.Boolean `nv:"value"`
}

func (p propClones) value() []string {
	clones := make([]string, len(p.Value))
	i := 0
	for clone, _ := range p.Value {
		clones[i] = clone
	}
	return clones
}

type propUint64 struct {
	Value uint64 `nv:"value"`
}

func (p propUint64) value() uint64 {
	return p.Value
}

type propUint64WithSource struct {
	Source string `nv:"source"`
	Value  uint64 `nv:"value"`
}

func (p propUint64WithSource) value() uint64 {
	return p.Value
}

type propString struct {
	Value string `nv:"value"`
}

func (p propString) value() string {
	return p.Value
}

type propStringWithSource struct {
	Source string `nv:"source"`
	Value  string `nv:"value"`
}

func (p propStringWithSource) value() string {
	return p.Value
}

func dsToDataset(in *ds) *Dataset {
	var dsType string
	if in.DMUObjsetStats.IsSnapshot {
		dsType = DatasetSnapshot
	} else if dmuType(in.Properties.Type.Value) == dmuTypes["zvol"] {
		dsType = DatasetVolume
	} else {
		dsType = DatasetFilesystem
	}

	compression := in.Properties.Compression.Value
	if compression == "" {
		compression = "off"
	}

	mountpoint := in.Properties.Mountpoint.Value
	if mountpoint == "" && dsType != DatasetSnapshot {
		mountpoint = fmt.Sprintf("/%s", in.Name)
	}

	return &Dataset{
		Name:          in.Name,
		Origin:        in.Properties.Origin.Value,
		Used:          in.Properties.Used.Value,
		Avail:         in.Properties.Available.Value,
		Mountpoint:    mountpoint,
		Compression:   compression,
		Type:          dsType,
		Written:       in.Properties.Available.Value,
		Volsize:       in.Properties.Volsize.Value,
		Usedbydataset: in.Properties.UsedByDataset.Value,
		Logicalused:   in.Properties.LogicalUsed.Value,
		Quota:         in.Properties.Quota.Value,
		ds:            in,
	}
}

func getDatasets(name, dsType string, recurse bool, depth uint64) ([]*Dataset, error) {
	types := map[string]bool{
		dsType: true,
	}

	dss, err := list(name, types, recurse, depth)
	if err != nil {
		return nil, err
	}

	datasets := make([]*Dataset, len(dss))
	for i, ds := range dss {
		datasets[i] = dsToDataset(ds)
	}

	return datasets, nil
}

// Datasets retrieves a list of all datasets, regardless of type
func Datasets(name string) ([]*Dataset, error) {
	return getDatasets(name, "all", true, 0)
}

// Filesystems retrieves a list of all filesystems
func Filesystems(name string) ([]*Dataset, error) {
	return getDatasets(name, DatasetFilesystem, true, 0)
}

// Snapshots retrieves a list of all snapshots
func Snapshots(name string) ([]*Dataset, error) {
	return getDatasets(name, DatasetSnapshot, true, 0)
}

// Volumes retrieves a list of all volumes
func Volumes(name string) ([]*Dataset, error) {
	return getDatasets(name, DatasetVolume, true, 0)
}

// GetDataset retrieves a single dataset
func GetDataset(name string) (*Dataset, error) {
	datasets, err := getDatasets(name, "all", false, 0)
	if err != nil {
		return nil, err
	}
	if len(datasets) != 1 {
		return nil, fmt.Errorf("expected 1 dataset, got %s", len(datasets))
	}
	return datasets[0], nil
}

func createDataset(name string, createType dmuType, properties map[string]interface{}) (*Dataset, error) {
	if err := create(name, dmuZFS, properties); err != nil {
		return nil, err
	}

	return GetDataset(name)
}

// CreateFilesystem creates a new filesystem
func CreateFilesystem(name string, properties map[string]interface{}) (*Dataset, error) {
	// TODO: Sort out handling of properties. Custom struct?
	return createDataset(name, dmuZFS, properties)
}

// CreateVolume creates a new volume
func CreateVolume(name string, size uint64, properties map[string]interface{}) (*Dataset, error) {
	// TODO: Sort out handling of properties. Custom struct?
	properties["volsize"] = size
	return createDataset(name, dmuZVOL, properties)
}

// ReceiveSnapshot creates a snapshot from a zfs send stream
func ReceiveSnapshot(input io.Reader, name string) (*Dataset, error) {
	// TODO: Fix when zfs receive is implemented
	return nil, errors.New("zfs receive not yet implemented")
}

// Children returns a list of children of the dataset
func (d *Dataset) Children(depth uint64) ([]*Dataset, error) {
	datasets, err := getDatasets(d.Name, "all", true, depth)
	if err != nil {
		return nil, err
	}
	return datasets[1:], nil
}

// Clones returns a list of clones of the dataset
func (d *Dataset) Clones()

func (d *Dataset) Clone(name string, properties map[string]interface{}) (*Dataset, error) {
	if err := clone(name, d.Name, properties); err != nil {
		return nil, err
	}
	return GetDataset(name)
}

type DestroyOptions struct {
	Recursive       bool
	RecursiveClones bool
	ForceUnmount    bool
	Defer           bool
}

// Destroy destroys a zfs dataset, optionally recursive for descendants and
// clones. Note that recursive destroys are not an atomic operation.
func (d *Dataset) Destroy(opts *DestroyOptions) error {
	// Recurse
	if opts.Recursive {
		children, err := d.Children(1)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := child.Destroy(opts); err != nil {
				return err
			}
		}
	}

	// Recurse Clones
	if opts.RecursiveClones {
		for cloneName, _ := range d.ds.Properties.Clones.Value {
			clone, err := GetDataset(cloneName)
			if err != nil {
				return err
			}
			if err := clone.Destroy(opts); err != nil {
				return err
			}
		}
	}

	// Unmount this dataset
	// TODO: Implement when we have unmount

	// Destroy this dataset
	return destroy(d.Name, opts.Defer)
}

func (d *Dataset) Diff(name string) {
	// TODO: Implement when we have a zfs diff implementation
}

func (d *Dataset) GetProperty(name string) (interface{}, error) {
	dV := reflect.ValueOf(d.ds.Properties)
	propertyIndex, ok := dsPropertyIndexes[strings.ToLower(name)]
	if !ok {
		return nil, errors.New("not a valid property name")
	}
	property := reflect.Indirect(dV).Field(propertyIndex).Interface().(dsProperty)
	return property.value(), nil
}

func (d *Dataset) SetProperty(name string, value interface{}) error {
	// TODO: Implement when we have a zfs set property implementation
	return errors.New("zfs set property not implemented yet")
}

func (d *Dataset) Rollback(destroyMoreRecent bool) error {
	// TODO: Handle the destroyMoreRecent option
	_, err := rollback(d.Name)
	return err
}

// TODO: Decide whether asking for a fd here instead of an io.Writer is ok
func (d *Dataset) SendSnapshot(outputFD uintptr) error {
	return send(d.Name, outputFD, "", false, false)
}

func (d *Dataset) Snapshot(name string, recursive bool) error {
	zpool := strings.Split(d.Name, "/")[0]
	snapName := fmt.Sprintf("%s@%s", d.Name, name)
	_, err := snapshot(zpool, []string{snapName}, map[string]string{})
	return err
}

func (d *Dataset) Snapshots() ([]*Dataset, error) {
	return Snapshots(d.Name)
}

func init() {
	dsPropertyIndexes = make(map[string]int)
	dsPropertiesT := reflect.TypeOf(dsProperties{})
	for i := 0; i < dsPropertiesT.NumField(); i++ {
		field := dsPropertiesT.Field(i)
		name := field.Name
		tags := strings.Split(field.Tag.Get("nv"), ",")
		if len(tags) > 0 && tags[0] != "" {
			name = tags[0]
		}
		dsPropertyIndexes[strings.ToLower(name)] = i
	}
}
