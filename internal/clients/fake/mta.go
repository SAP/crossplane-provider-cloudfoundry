package fake

import (
	"net/http"

	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"
	"github.com/cloudfoundry-incubator/multiapps-cli-plugin/util"
	"github.com/stretchr/testify/mock"

	mtaModels "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/models"
)

// MockMTA mocks App interfaces
type MockMTA struct {
	mock.Mock
}

// GetMta mocks Mta.GetMta
func (m *MockMTA) GetMta(guid string) (*mtaModels.Mta, error) {
	args := m.Called()
	return args.Get(0).(*mtaModels.Mta), args.Error(1)
}

// StartMtaOperation mocks Mta.StartMtaOperation
func (m *MockMTA) StartMtaOperation(operation mtaModels.Operation) (mtaClient.ResponseHeader, error) {
	args := m.Called()
	return args.Get(0).(mtaClient.ResponseHeader), args.Error(1)
}

// GetMtaOperation mocks Mta.GetMtaOperation
func (m *MockMTA) GetMtaOperation(operationID, embed string) (*mtaModels.Operation, error) {
	args := m.Called()
	return args.Get(0).(*mtaModels.Operation), args.Error(1)
}

// UploadMtaFile mocks Mta.UploadMtaFile
func (m *MockMTA) UploadMtaFile(file util.NamedReadSeeker, fileSize int64, namespace *string) (*mtaModels.FileMetadata, error) {
	args := m.Called()
	return args.Get(0).(*mtaModels.FileMetadata), args.Error(1)
}

// StartUploadMtaArchiveFromUrl mocks Mta.StartUploadMtaArchiveFromUrl
func (m *MockMTA) StartUploadMtaArchiveFromUrl(fileUrl string, namespace *string) (http.Header, error) {
	args := m.Called()
	return args.Get(0).(http.Header), args.Error(1)
}

// GetAsyncUploadJob mocks Mta.GetAsyncUploadJob
func (m *MockMTA) GetAsyncUploadJob(jobId string, namespace *string, appInstanceId string) (mtaClient.AsyncUploadJobResult, error) {
	args := m.Called()
	return args.Get(0).(mtaClient.AsyncUploadJobResult), args.Error(1)
}

// Mta is a nil Mta
var (
	MtaNil *mtaModels.Mta
)

// Mta is a Mta object
type Mta struct {
	mtaModels.Mta
}

// NewMta generate a new Mta
func NewMta() *Mta {
	r := &Mta{}
	return r
}

// SetMetadataID assigns MTA ID
func (a *Mta) SetMetadataID(guid string) *Mta {
	a.Metadata = &mtaModels.Metadata{ID: guid}
	return a
}
