package mta

import (
	"encoding/base64"
	"fmt"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"
	"io"
	v1 "k8s.io/api/core/v1"
	"net/url"
	"path"
	"strings"

	"github.com/pkg/errors"

	mtaModels "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/models"
)

var nonProtectedMethods = map[string]struct{}{"GET": {}, "HEAD": {}, "TRACE": {}, "OPTIONS": {}}

const deploy_service_host = "deploy-service"

const (
	// error messages
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

func Exists(cr *v1alpha1.Mta, c mtaClient.MtaClientOperations) (bool, error) {
	_, err := c.GetMta(*cr.Status.AtProvider.MtaId)
	if err != nil {
		if strings.Contains(err.Error(), notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func Observe(cr *v1alpha1.Mta, c mtaClient.MtaClientOperations) (v1alpha1.MtaObservation, error) {
	observation, err := observeMta(cr, c)
	if err != nil {
		return observation, err
	}

	fileObservations := []v1alpha1.FileObservation{}
	for _, file := range *cr.Status.AtProvider.Files {
		fileObservation, mtaId, err := observeFile(cr, file, c)
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

func Deploy(cr *v1alpha1.Mta, c mtaClient.MtaClientOperations) (v1alpha1.MtaObservation, error) {
	fileObservation := cr.FindFileObservation(cr.Spec.ForProvider.File)
	if fileObservation == nil || fileObservation.ID == nil {
		// file is not available yet, wait for it
		return cr.Status.AtProvider, nil
	}

	processType := "DEPLOY"
	parameters := map[string]interface{}{
		"mtaId":        cr.Status.AtProvider.MtaId,
		"appArchiveId": fileObservation.ID,
		"versionRule":  cr.Spec.ForProvider.VersionRule,
		"abortOnError": cr.Spec.ForProvider.AbortOnError,
		"modules":      cr.Spec.ForProvider.Modules,
	}

	blueGreen := *cr.Spec.ForProvider.BlueGreenDeploy
	if blueGreen {
		processType = "BLUE_GREEN_DEPLOY"
		parameters["noConfirm"] = "true"
		parameters["skipIdleStart"] = "true"
		parameters["keepOriginalAppNamesAfterDeploy"] = "true"
		parameters["shouldApplyIncrementalInstancesUpdate"] = "false"
		parameters["shouldBackupPreviousVersion"] = "false"
	}

	// Noch nicht fertig, ist ein Logikfehler bei LastOperation
	if cr.Spec.ForProvider.AbortOnError != nil {
		if !*cr.Spec.ForProvider.AbortOnError {
			if cr.Status.AtProvider.LastOperation == nil || cr.Status.AtProvider.LastOperation.HasError() {
				return v1alpha1.MtaObservation{}, errors.New("Deployment aborted: MTA file contains errors")
			}
		}
		parameters["abortOnError"] = *cr.Spec.ForProvider.AbortOnError
	}

	// Fertig
	if cr.Spec.ForProvider.VersionRule != nil {
		vaildVersionRules := map[string]bool{
			"HIGHER":      true,
			"SAME_HIGHER": true,
			"ALL":         true,
		}
		if vaildVersionRules[*cr.Spec.ForProvider.VersionRule] {
			parameters["versionRule"] = *cr.Spec.ForProvider.VersionRule
		}
	}

	// Fertig
	if cr.Spec.ForProvider.Modules != nil {
		parameters["modulesForDeployment"] = strings.Join(*cr.Spec.ForProvider.Modules, ",")
	}

	if *cr.Status.AtProvider.MtaExtensionId != "" {
		parameters["mtaExtDescriptorId"] = cr.Status.AtProvider.MtaExtensionId
	}

	operation := mtaModels.Operation{
		ProcessType: processType,
		Namespace:   *cr.Spec.ForProvider.Namespace,
		Parameters:  parameters,
	}

	responseHeaders, err := c.StartMtaOperation(operation)
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

func Delete(cr *v1alpha1.Mta, c mtaClient.MtaClientOperations) (v1alpha1.MtaObservation, error) {
	operation := mtaModels.Operation{
		ProcessType: "UNDEPLOY",
		Namespace:   *cr.Spec.ForProvider.Namespace,
		Parameters: map[string]interface{}{
			"mtaId":          cr.Status.AtProvider.MtaId,
			"deleteServices": true,
			"abortOnError":   "true",
		},
	}

	responseHeaders, err := c.StartMtaOperation(operation)
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

func CreateExtensions(cr *v1alpha1.Mta, o *v1alpha1.MtaObservation, c mtaClient.MtaClientOperations) error {
	if cr.HasExtension() && !cr.IsExtensionAlreadyUploaded() {
		stringReader := strings.NewReader(*cr.Spec.ForProvider.Extension)
		reader := NamedReadSeeker{stringReader, "extension.mtaext"}

		mtaExtFileMetadata, err := c.UploadMtaFile(reader, stringReader.Size(), cr.Spec.ForProvider.Namespace)
		if err != nil {
			return err
		}

		o.MtaExtensionId = &mtaExtFileMetadata.ID
		o.MtaExtensionHash = &mtaExtFileMetadata.Digest
	}

	return nil
}

func UploadFileFromUrl(cr *v1alpha1.Mta, file *v1alpha1.File, secret *v1.Secret, c mtaClient.MtaClientOperations) (v1alpha1.FileObservation, error) {
	urlObj, err := url.Parse(*file.URL)
	if err != nil {
		return v1alpha1.FileObservation{}, errors.Wrap(err, errNoUrl)
	}

	if secret != nil {
		urlObj.User = url.UserPassword(string(secret.Data["user"]), string(secret.Data["password"]))
	}

	encodedUrl := base64.StdEncoding.EncodeToString([]byte(urlObj.String()))
	responseHeaders, err := c.StartUploadMtaArchiveFromUrl(encodedUrl, cr.Spec.ForProvider.Namespace)
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

func observeFile(cr *v1alpha1.Mta, file v1alpha1.FileObservation, c mtaClient.MtaClientOperations) (v1alpha1.FileObservation, *string, error) {
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

	jobResult, err := c.GetAsyncUploadJob(*observation.LastOperation.ID, cr.Spec.ForProvider.Namespace, *observation.AppInstance)
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

func observeMta(cr *v1alpha1.Mta, c mtaClient.MtaClientOperations) (v1alpha1.MtaObservation, error) {
	observation := v1alpha1.MtaObservation{
		MtaId:            cr.Status.AtProvider.MtaId,
		MtaExtensionId:   cr.Status.AtProvider.MtaExtensionId,
		MtaExtensionHash: cr.Status.AtProvider.MtaExtensionHash,
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

	o, err := c.GetMtaOperation(*observation.LastOperation.ID, "messages")
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
