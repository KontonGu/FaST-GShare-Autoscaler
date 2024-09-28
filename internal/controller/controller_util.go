/*
Copyright 2024 FaST-GShare Authors, KontonGu (Jianfeng Gu), et. al.
@Techinical University of Munich, CAPS Cloud Team

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"regexp"
	"strconv"
)

func getResKeyName(quota int64, smPartition int64) string {
	return "-q" + strconv.Itoa(int(quota)) + "-p" + strconv.Itoa(int(smPartition))
}

func parseFromKeyName(key string) (int, int) {
	r := regexp.MustCompile(`-q(\d+)-p(\d+)`)
	match := r.FindStringSubmatch(key)
	if match == nil {
		return 0, 0
	}
	quota, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, 0
	}
	partition, err := strconv.Atoi(match[2])
	if err != nil {
		return 0, 0
	}
	return quota, partition
}