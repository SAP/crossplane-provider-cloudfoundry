package v1alpha1

import (
	// #nosec G501: Blocklisted import crypto/md5: weak cryptographic primitive
	"crypto/md5"
	"fmt"
	"reflect"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// MtaParameters are the configurable fields of a Mta.
type MtaParameters struct {
	// (Bool) Use blue-green deployment
	// +kubebuilder:validation:Optional
	BlueGreenDeploy *bool `json:"blueGreenDeploy,omitempty"`

	// (String) The URL of the deploy service, if a custom one has been used(should be present in the same landscape). By default 'deploy-service.<system-domain>'
	// The URL of the deploy service, if a custom one has been used(should be present in the same landscape). By default 'deploy-service.<system-domain>'
	// +kubebuilder:validation:Optional
	DeployURL *string `json:"deployUrl,omitempty"`

	// (String) The namespace of the MTA. Should be of valid host format
	// The namespace of the MTA. Should be of valid host format
	// +kubebuilder:validation:Optional
	Namespace *string `json:"namespace,omitempty"`

	// Reference to a Space in space to populate space.
	SpaceReference `json:",inline"`

	// +kubebuilder:validation:Required
	File *File `json:"file"`

	Extension *string `json:"extension,omitempty"`

	// (Bool) Specifies whether the deployment should be aborted if an error occurs
	// +kubebuilder:validation:Optional
	AbortOnError *bool `json:"abortOnError,omitempty"`

	// Specifies the versioning rule to be applied for the resource
	// +kubebuilder-validation:Enum=HIGHER;SAME_HIGHER;ALL
	VersionRule *string `json:"versionRule,omitempty"`

	// Deploy only the modules of the MTA with the specified names. If not specified, all modules are deployed.
	// +kubebuilder-validation:Optional
	Modules *[]string `json:"modules,omitempty"`

	// (Bool) Specifies whether to re-create changed services and delete discontinued services.
	// +kubebuilder:validation:Optional
	DeleteServices *bool `json:"deleteServices,omitempty"`
}

type FileObservation struct {
	ID *string `json:"id,omitempty"`

	AppInstance *string `json:"appInstance,omitempty"`

	URL *string `json:"url,omitempty"`

	LastOperation *Operation `json:"operation,omitempty"`
}

// MtaObservation are the observable fields of a Mta.
type MtaObservation struct {
	MtaId *string `json:"mtaId,omitempty"`

	MtaExtensionId *string `json:"mtaExtensionId,omitempty"`

	MtaExtensionHash *string `json:"mtaExtensionHash,omitempty"`

	MtaModules *[]string `json:"mtaModulesForDeployment,omitempty"`

	Files *[]FileObservation `json:"files,omitempty"`

	LastOperation *Operation `json:"lastOperation,omitempty"`
}

type File struct {
	// Reference to a secret containing a user and an optional password, which is added to the URL of the MTA.
	// +kubebuilder:validation:Optional
	CredentialsSecretRef *xpv1.SecretReference `json:"credentialsSecretRef,omitempty"`

	// (String) The remote URL where the MTA archive is present
	// The remote URL where the MTA archive is present
	// +kubebuilder:validation:Required
	URL *string `json:"url,omitempty"`
}

// A MtaSpec defines the desired state of a Mta.
type MtaSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       MtaParameters `json:"forProvider"`
}

// A MtaStatus represents the observed state of a Mta.
type MtaStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          MtaObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Mta is an example API type.
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Synced",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="External-Name",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,crossplaneprovidermta}
type Mta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MtaSpec   `json:"spec"`
	Status MtaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MtaList contains a list of Mta
type MtaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mta `json:"items"`
}

// Mta type metadata.
var (
	MtaKind             = reflect.TypeOf(Mta{}).Name()
	MtaGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: MtaKind}.String()
	MtaKindAPIVersion   = MtaKind + "." + CRDGroupVersion.String()
	MtaGroupVersionKind = CRDGroupVersion.WithKind(MtaKind)
)

func init() {
	SchemeBuilder.Register(&Mta{}, &MtaList{})
}

func (m *Mta) AllFiles() []File {
	files := []File{}

	if m.Spec.ForProvider.File != nil {
		files = append(files, *m.Spec.ForProvider.File)
	}

	return files
}

func (m *Mta) HasExtension() bool {
	return m.Spec.ForProvider.Extension != nil
}

func (m *Mta) IsExtensionAlreadyUploaded() bool {
	return m.Status.AtProvider.MtaExtensionId != nil
}

func (m *Mta) AreModulesApplied() bool {
	return m.Status.AtProvider.MtaModules != nil
}

func (m *Mta) HasChangedExtension() bool {
	var desired string
	if m.Spec.ForProvider.Extension != nil {
		// #nosec G401: Use of weak cryptographic primitive
		desired = fmt.Sprintf("%x", md5.Sum([]byte(*m.Spec.ForProvider.Extension)))
	} else {
		desired = ""
	}

	var actual string
	if m.Status.AtProvider.MtaExtensionHash != nil {
		actual = *m.Status.AtProvider.MtaExtensionHash
	} else {
		actual = ""
	}

	if !strings.EqualFold(desired, actual) {
		return true
	}
	return false
}

func (m *Mta) HaveDeploymentModulesChanged() bool {
	return !reflect.DeepEqual(m.Spec.ForProvider.Modules, m.Status.AtProvider.MtaModules)
}

func (m *Mta) FindFileObservation(file *File) *FileObservation {
	if m.Status.AtProvider.Files == nil {
		return nil
	}

	for _, f := range *m.Status.AtProvider.Files {
		if f.URL != nil && *f.URL == *file.URL {
			return &f
		}
	}

	return nil
}

func (m *Mta) HasChangedUrls() bool {
	files := m.AllFiles()

	for _, file := range files {
		fileCopy := file
		if m.FindFileObservation(&fileCopy) == nil {
			return true
		}
	}

	return false
}

func (m *Mta) HasRunningOperation() bool {
	return slices.ContainsFunc(m.allOperations(), func(operation Operation) bool {
		return operation.IsRunning()
	})
}

func (m *Mta) HasErrorOperation() bool {
	return slices.ContainsFunc(m.allOperations(), func(operation Operation) bool {
		return operation.HasError() || operation.isAborted()
	})
}

func (m *Mta) GetErrorOperation() string {
	operations := m.allOperations()

	errIndex := slices.IndexFunc(operations, func(operation Operation) bool {
		return operation.HasError() || operation.isAborted()
	})

	return operations[errIndex].GetError()
}

func (m *Mta) allOperations() []Operation {
	operations := []Operation{}

	if m.Status.AtProvider.LastOperation != nil && m.Status.AtProvider.LastOperation.ID != nil {
		operations = append(operations, *m.Status.AtProvider.LastOperation)
	}

	if m.Status.AtProvider.Files == nil {
		return operations
	}

	for _, v := range *m.Status.AtProvider.Files {
		if v.LastOperation != nil && v.LastOperation.ID != nil {
			operations = append(operations, *v.LastOperation)
		}
	}

	return operations
}

// implement SpaceScoped interface
func (m *Mta) GetSpaceRef() *SpaceReference {
	return &m.Spec.ForProvider.SpaceReference
}
