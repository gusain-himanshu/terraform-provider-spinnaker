package api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/mitchellh/mapstructure"
	gate "github.com/spinnaker/spin/cmd/gateclient"
	gateapi "github.com/spinnaker/spin/gateapi"
)

func CreatePipeline(client *gate.GatewayClient, pipelineJson map[string]interface{}) error {
	application := pipelineJson["application"].(string)
	pipelineName := pipelineJson["name"].(string)
	foundPipeline, queryResp, _ := client.ApplicationControllerApi.GetPipelineConfigUsingGET(client.Context, application, pipelineName)
	switch queryResp.StatusCode {
	case http.StatusOK:
		// pipeline found, let's use Spinnaker's known Pipeline ID, otherwise we'll get one created for us
		if len(foundPipeline) > 0 {
			tflog.Debug(client.Context, "pipeline already exists", pipelineJson)
			pipelineJson["id"] = foundPipeline["id"].(string)
		}
	case http.StatusNotFound:
		// pipeline doesn't exists, let's create a new one
	default:
		b, _ := io.ReadAll(queryResp.Body)
		return fmt.Errorf("unhandled response %d: %s", queryResp.StatusCode, b)
	}

	// TODO: support option passing in and remove nil in below call
	opt := &gateapi.PipelineControllerApiSavePipelineUsingPOSTOpts{}
	saveResp, err := client.PipelineControllerApi.SavePipelineUsingPOST(client.Context, pipelineJson, opt)
	if err != nil {
		return err
	}
	if saveResp.StatusCode != http.StatusOK {
		return fmt.Errorf("encountered an error saving pipeline, status code: %d", saveResp.StatusCode)
	}
	return nil
}

func GetPipeline(client *gate.GatewayClient, applicationName, pipelineName string, dest interface{}) (map[string]interface{}, error) {
	jsonMap, resp, err := client.ApplicationControllerApi.GetPipelineConfigUsingGET(client.Context,
		applicationName,
		pipelineName)

	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return jsonMap, ErrCodeNoSuchEntityException
		}
		return jsonMap, fmt.Errorf("encountered an error getting pipeline %s, %s",
			pipelineName,
			err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return jsonMap, fmt.Errorf("encountered an error getting pipeline in pipeline %s with name %s, status code: %d",
			applicationName,
			pipelineName,
			resp.StatusCode)
	}

	if jsonMap == nil {
		return nil, ErrCodeNoSuchEntityException
	}

	if err := mapstructure.Decode(jsonMap, dest); err != nil {
		return jsonMap, err
	}

	return jsonMap, nil
}

func UpdatePipeline(client *gate.GatewayClient, pipelineID string, pipeline interface{}) error {
	_, resp, err := client.PipelineControllerApi.UpdatePipelineUsingPUT(client.Context, pipelineID, pipeline)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("encountered an error saving pipeline, status code: %d", resp.StatusCode)
	}

	return nil
}

func DeletePipeline(client *gate.GatewayClient, applicationName, pipelineName string) error {
	resp, err := client.PipelineControllerApi.DeletePipelineUsingDELETE(client.Context, applicationName, pipelineName)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("encountered an error deleting pipeline, status code: %d", resp.StatusCode)
	}

	return nil
}
