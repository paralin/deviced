package utils

import (
	"strings"

	dc "github.com/fsouza/go-dockerclient"
)

func BuildImageMap(images *[]dc.APIImages) map[string][]string {
	// Map of image name -> available tag list
	availableTagMap := map[string][]string{}
	for _, img := range *images {
		for _, tagfull := range img.RepoTags {
			if strings.Contains(tagfull, "<none>") {
				continue
			}
			image, tag := ParseImageAndTag(tagfull)
			tagList := availableTagMap[image]
			tagList = append(tagList, tag)
			availableTagMap[image] = tagList
		}
	}
	return availableTagMap
}

func ParseImageAndTag(imagestr string) (string, string) {
	imagePts := strings.Split(imagestr, ":")
	image := imagePts[0]
	var imageTag string
	if len(imagePts) < 2 {
		imageTag = "latest"
	} else {
		imageTag = imagePts[1]
	}
	return image, imageTag
}
