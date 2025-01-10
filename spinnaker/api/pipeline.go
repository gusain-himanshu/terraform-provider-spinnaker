package api

import (
	b64 "encoding/base64"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/mapstructure"
	gate "github.com/spinnaker/spin/cmd/gateclient"
	orca_tasks "github.com/spinnaker/spin/cmd/orca-tasks"
)

type CreatePipeLineTask map[string]interface{}

func NewSavePipelineTask(d *schema.ResourceData) (CreatePipeLineTask, error) {
	pipeLineTask := make(map[string]interface{})
	pipeLineTask["application"] = d.Get("application").(string)
	pipeLineTask["description"] = fmt.Sprintf("Save Pipeline %s", d.Get("name").(string))
	pipeLineTask["job"] = []map[string]interface{}{
		{
			"type":     "savePipeline",
			"pipeline": b64.StdEncoding.EncodeToString([]byte(d.Get("pipeline").(string))),
		},
	}
	return pipeLineTask, nil
}

// CreatePipeline creates passed pipeline
func CretePipeLineWithTask(client *gate.GatewayClient, createPipeLineTask CreatePipeLineTask) error {
	ref, _, err := client.TaskControllerApi.TaskUsingPOST1(client.Context, createPipeLineTask)
	if err != nil {
		return err
	}
	return orca_tasks.WaitForSuccessfulTask(client, ref)
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
