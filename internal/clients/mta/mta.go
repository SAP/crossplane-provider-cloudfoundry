package mta

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"

	// TODO exchange incubator with cloudfoundry itself
	mtaModels "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/models"
	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"
	"github.com/cloudfoundry-incubator/multiapps-cli-plugin/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/pkg/errors"
)

type MtaClient interface {
	GetMta(mtaID string) (*mtaModels.Mta, error)
	StartMtaOperation(operation mtaModels.Operation) (mtaClient.ResponseHeader, error)
	GetMtaOperation(operationID, embed string) (*mtaModels.Operation, error)
	UploadMtaFile(file util.NamedReadSeeker, fileSize int64, namespace *string) (*mtaModels.FileMetadata, error)
	StartUploadMtaArchiveFromUrl(fileUrl string, namespace *string) (http.Header, error)
	GetAsyncUploadJob(jobId string, namespace *string, appInstanceId string) (mtaClient.AsyncUploadJobResult, error)
}

type Client struct {
	MtaClient MtaClient
}

// Error messages
const (
	errParseUrl      = "cannot parse CF API URL"
	errNoUrl         = "could not parse mtarUrl"
	notFound         = "404 Not Found"
	asyncJobNotFound = "could not get async file upload job"
)

type NamedReadSeeker struct {
	io.ReadSeeker
	FileName string
}

func (t NamedReadSeeker) Name() string {
	return t.FileName
}

func (c *Client) Exists(cr *v1alpha1.Mta) (bool, error) {
	_, err := c.MtaClient.GetMta(*cr.Status.AtProvider.MtaId)
	if err != nil {
		if strings.Contains(err.Error(), notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) Observe(cr *v1alpha1.Mta) (v1alpha1.MtaObservation, error) {
	observation, err := c.observeMta(cr)
	if err != nil {
		return observation, err
	}

	// TODO Timeout Function so we pause until the file is uploaded
	fileObservations := []v1alpha1.FileObservation{}
	for _, file := range *cr.Status.AtProvider.Files {
		fileObservation, mtaId, err := c.observeFile(cr, file)
		if err != nil {
			return observation, err
		}

		if observation.MtaId != mtaId && mtaId != nil {
			observation.MtaId = mtaId
		}

		fileObservations = append(fileObservations, fileObservation)
	}
	observation.Files = &fileObservations

	return observation, nil
}

func (c *Client) Deploy(cr *v1alpha1.Mta) (v1alpha1.MtaObservation, error) {
	fileObservation := cr.FindFileObservation(cr.Spec.ForProvider.File)
	if fileObservation == nil || fileObservation.ID == nil {
		// file is not available yet, wait for it
		return cr.Status.AtProvider, nil
	}

	processType := "DEPLOY"
	parameters := map[string]interface{}{
		"mtaId":        cr.Status.AtProvider.MtaId,
		"appArchiveId": fileObservation.ID,
	}

	blueGreen := ptr.Deref(cr.Spec.ForProvider.BlueGreenDeploy, false)
	if blueGreen {
		processType = "BLUE_GREEN_DEPLOY"
		parameters["noConfirm"] = "true"
		parameters["skipIdleStart"] = "true"
		parameters["keepOriginalAppNamesAfterDeploy"] = "true"
		parameters["shouldApplyIncrementalInstancesUpdate"] = "false"
		parameters["shouldBackupPreviousVersion"] = "false"
	}

	abortOnError := ptr.Deref(cr.Spec.ForProvider.AbortOnError, false)
	if abortOnError {
		parameters["abortOnError"] = abortOnError
	}

	deleteServices := ptr.Deref(cr.Spec.ForProvider.DeleteServices, false)
	if abortOnError {
		parameters["deleteServices"] = deleteServices
	}

	if cr.Spec.ForProvider.VersionRule != nil {
		parameters["versionRule"] = *cr.Spec.ForProvider.VersionRule
	}

	if cr.Spec.ForProvider.Modules != nil {
		parameters["modulesForDeployment"] = strings.Join(*cr.Spec.ForProvider.Modules, ",")
	}

	mtaExtensionId := ptr.Deref(cr.Status.AtProvider.MtaExtensionId, "")
	if mtaExtensionId != "" {
		parameters["mtaExtDescriptorId"] = cr.Status.AtProvider.MtaExtensionId
	}

	namespace := ptr.Deref(cr.Spec.ForProvider.Namespace, "default")
	operation := mtaModels.Operation{
		ProcessType: processType,
		Namespace:   namespace,
		Parameters:  parameters,
	}

	responseHeaders, err := c.MtaClient.StartMtaOperation(operation)
	if err != nil {
		return v1alpha1.MtaObservation{}, err
	}

	operationId, err := getOperationIdFromLocationHeader(responseHeaders.Location.String())
	if err != nil {
		return v1alpha1.MtaObservation{}, err
	}

	return v1alpha1.MtaObservation{
		MtaId:            cr.Status.AtProvider.MtaId,
		MtaExtensionId:   cr.Status.AtProvider.MtaExtensionId,
		MtaExtensionHash: cr.Status.AtProvider.MtaExtensionHash,
		Files:            cr.Status.AtProvider.Files,
		MtaModules:       cr.Status.AtProvider.MtaModules,
		LastOperation: &v1alpha1.Operation{
			ID: &operationId,
		},
	}, nil
}

func (c *Client) Delete(cr *v1alpha1.Mta) (v1alpha1.MtaObservation, error) {
	namespace := ptr.Deref(cr.Spec.ForProvider.Namespace, "default")
	operation := mtaModels.Operation{
		ProcessType: "UNDEPLOY",
		Namespace:   namespace,
		Parameters: map[string]interface{}{
			"mtaId":          cr.Status.AtProvider.MtaId,
			"deleteServices": true,
			"abortOnError":   "true",
		},
	}

	responseHeaders, err := c.MtaClient.StartMtaOperation(operation)
	if err != nil {
		return v1alpha1.MtaObservation{}, err
	}

	operationId, err := getOperationIdFromLocationHeader(responseHeaders.Location.String())
	if err != nil {
		return v1alpha1.MtaObservation{}, err
	}

	return v1alpha1.MtaObservation{
		MtaId:            cr.Status.AtProvider.MtaId,
		MtaExtensionId:   cr.Status.AtProvider.MtaExtensionId,
		MtaExtensionHash: cr.Status.AtProvider.MtaExtensionHash,
		Files:            cr.Status.AtProvider.Files,
		LastOperation: &v1alpha1.Operation{
			ID: &operationId,
		},
	}, nil
}

func (c *Client) CreateExtensions(cr *v1alpha1.Mta, o *v1alpha1.MtaObservation) error {
	if cr.HasExtension() && !cr.IsExtensionAlreadyUploaded() {
		stringReader := strings.NewReader(*cr.Spec.ForProvider.Extension)
		reader := NamedReadSeeker{stringReader, "extension.mtaext"}

		mtaExtFileMetadata, err := c.MtaClient.UploadMtaFile(reader, stringReader.Size(), cr.Spec.ForProvider.Namespace)
		if err != nil {
			return err
		}

		o.MtaExtensionId = &mtaExtFileMetadata.ID
		o.MtaExtensionHash = &mtaExtFileMetadata.Digest
	}

	return nil
}

func ApplyModules(cr *v1alpha1.Mta, o *v1alpha1.MtaObservation) {
	if !cr.AreModulesApplied() {
		o.MtaModules = cr.Spec.ForProvider.Modules
	}
}

func (c *Client) UploadFileFromUrl(cr *v1alpha1.Mta, file *v1alpha1.File, secret *v1.Secret) (v1alpha1.FileObservation, error) {
	urlObj, err := url.Parse(*file.URL)
	if err != nil {
		return v1alpha1.FileObservation{}, errors.Wrap(err, errNoUrl)
	}

	if secret != nil {
		urlObj.User = url.UserPassword(string(secret.Data["user"]), string(secret.Data["password"]))
	}

	encodedUrl := base64.StdEncoding.EncodeToString([]byte(urlObj.String()))
	responseHeaders, err := c.MtaClient.StartUploadMtaArchiveFromUrl(encodedUrl, cr.Spec.ForProvider.Namespace)
	if err != nil {
		return v1alpha1.FileObservation{}, err
	}

	appInstance := responseHeaders.Get("x-cf-app-instance")
	location := responseHeaders.Get("Location")
	operationId, err := getOperationIdFromLocationHeader(location)
	if err != nil {
		return v1alpha1.FileObservation{}, err
	}

	return v1alpha1.FileObservation{
		AppInstance: &appInstance,
		URL:         file.URL,
		LastOperation: &v1alpha1.Operation{
			ID: &operationId,
		},
	}, nil
}

func (c *Client) observeFile(cr *v1alpha1.Mta, file v1alpha1.FileObservation) (v1alpha1.FileObservation, *string, error) {
	observation := v1alpha1.FileObservation{
		ID:          file.ID,
		URL:         file.URL,
		AppInstance: file.AppInstance,
		LastOperation: &v1alpha1.Operation{
			ID: file.LastOperation.ID,
		},
	}

	if !file.LastOperation.IsRunning() {
		// if operation is ended once, we don't need to observe it again
		observation.LastOperation = file.LastOperation

		return observation, cr.Status.AtProvider.MtaId, nil
	}

	jobResult, err := c.MtaClient.GetAsyncUploadJob(*observation.LastOperation.ID, cr.Spec.ForProvider.Namespace, *observation.AppInstance)
	if err != nil {
		return observation, cr.Status.AtProvider.MtaId, err
	}

	observation.LastOperation.Error = &jobResult.Error
	observation.LastOperation.State = &jobResult.Status

	if jobResult.File != nil {
		observation.ID = &jobResult.File.ID
	}

	return observation, &jobResult.MtaId, nil
}

func (c *Client) observeMta(cr *v1alpha1.Mta) (v1alpha1.MtaObservation, error) {
	observation := v1alpha1.MtaObservation{
		MtaId:            cr.Status.AtProvider.MtaId,
		MtaExtensionId:   cr.Status.AtProvider.MtaExtensionId,
		MtaExtensionHash: cr.Status.AtProvider.MtaExtensionHash,
		MtaModules:       cr.Status.AtProvider.MtaModules,
		Files:            cr.Status.AtProvider.Files,
	}

	if cr.Status.AtProvider.LastOperation == nil {
		return observation, nil
	}

	if !cr.Status.AtProvider.LastOperation.IsRunning() {
		// if operation is ended once, we don't need to observe it again
		observation.LastOperation = cr.Status.AtProvider.LastOperation

		return observation, nil
	}

	observation.LastOperation = &v1alpha1.Operation{
		ID: cr.Status.AtProvider.LastOperation.ID,
	}

	o, err := c.MtaClient.GetMtaOperation(*observation.LastOperation.ID, "messages")
	if err != nil {
		return observation, err
	}

	var errorMessage string
	if string(o.State) == string(mtaModels.StateERROR) {
		if messageCount := len(o.Messages); messageCount > 0 {
			errorMessage = fmt.Sprintf("last message %s", o.Messages[messageCount-1].Text)
		}
		errorMessage = fmt.Sprintf("operation failed with errorType %s", o.ErrorType)
	}

	observation.LastOperation.Error = &errorMessage
	observation.LastOperation.State = (*string)(&o.State)

	return observation, nil
}

func getOperationIdFromLocationHeader(location string) (string, error) {
	urlObj, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if strings.HasSuffix(path.Dir(urlObj.Path), "operations") || strings.HasSuffix(path.Dir(urlObj.Path), "jobs") {
		return path.Base(urlObj.Path), nil
	}

	return "", nil
}
