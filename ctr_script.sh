#!/bin/bash -e
set -e
echo "3gb-1-layer-no-concurrency-no-parallelism" >> results.json

declare -a pull_times
declare -a speeds
declare -a memories

echo "[ {" >> results.json
# for i in $(seq 1 4); do
for j in $(seq 1 10); do
  # >&2 echo "Run: $j with parallel arg: $i"
  ECR_PULL_PARALLEL=0
  >&2 sudo service containerd stop
  >&2 sudo rm -rf /var/lib/containerd
  >&2 sudo mkdir -p /var/lib/containerd
  >&2 sudo service containerd start
  CGROUP_PARENT="ecr-pull-benchmark"
  CGROUP_CHILD="count-$j-parallel-${ECR_PULL_PARALLEL}-slice"
  CGROUP=${CGROUP_PARENT}/${CGROUP_CHILD}
  IMAGE_URL="020023120753.dkr.ecr.us-west-1.amazonaws.com/3gb-single:latest"
  TOKEN="eyJwYXlsb2FkIjoiVlFLeGFML1UvaFdncXNsOWx1Y2lZUTdMZkZkVG4xazVaRW14bFBkNHdIeWhhMWprY2lVTjlUT1dQNGUwVlM0TDJGN2hIMTdSWmd3NEVQMGd2VVVJWlB4WERxVmwxd24yOWp4SzhFdDBDRXJ4Y0w5aDlRckRPdjg1L0JBdFN0SW9VazUwZWQxNUxFTGl5M1FNUUoxN0hqR1F0dFdxRFB2RlpLQUpVN1VNYnpJSWtjcUZhbGU2OThNTnIwTWxHdFhEbDdNYjAvQ1dzVjFhU3d2REJVYzA2RWlaY1hFL3RuM3R2N0pqdEx0alc3aXQwSGN6Q2gvKzBJOHpoSm9IUTBwVkNXN0t5N2pyTC9CNU9Wblh5MnYxTDVqWjJJblhWaTc3Q3pLVnJjemNJcHdHTmZDc1lPQmh3em5ydTZMakk1ZFlpQXJlUkZ5YlF6QnBwaDN2YkpsL0NpV3liUGpCa3Z2enBqdTlrVjM1RnI2TjNkaGRmTGkvSUZPMFpqTWNIMTJMNnJjc1N3T2ZEYjJ5RWdnUnRkZklUYStmSWtwZHFIWXBTVnZYbXNLcUFsVU1Wb0pQVDM3MmltdFJJSU9ZUWgremRqUlJ5eGlqMWoyV0Jwb3A3cGxYc1FPNmcwSmd6SmxUYlpUZ0ExeXM5amVGR1lIMXZKb1RzWEo2UzdyY1pGUVorWFI5ZHFidjQ3cVRYNzVEditJM2JwYkpNcFErV0Nad2NIci9kMUltdzdZcXVBWXNwV0ZqaE0yWmU4bjk3R016dTVJaXFmMzB5UE9jcWdkRXdlNzhrWG1SL2c3YUgvOG1KZ3VzdXhHWGtqY0VOd1hwTnJId21QY003OEE5amZxQUdJOG4yaG4vcTdnc25KaCsvRU5oV1F1K0kvd2pFQ2toeVZoYWkycFJ0Ky93UEhQeTlPRWhMZVYvd0ZKZW5XMml0ekV6dE8xRzRaclY5RXRoV3FBQVlGeVNLRGxJSGVVQmhKTXhsa21mK05zQ1JLTmFnYnNqMXFteERPdjVyaDFwc2wvQ0Z3Qk9MTlNqWHEwQURISmdBaHRxYXRkUW4yVGJtdTBSTEs5MU5wVkZ1T2RweCtlbEpIM3I1dXhHWnhMSk5BNXdVdTl5ZnRHcCtpcmZKQVBmNGVkS005K1RHZXU1Uk14eEVxZ2F1Nk40eHNDOXV1S0ZiU0pEZ3lJK2pleU42a3RMaHVrbG9nUmNlWlJCV2hIRUlvTTRZOW5OOHZLNjFLMFg1VkpkWWpXb0QvT2MxSDU2UXV5S0loaG9KcFh2YmhwWEQ2Tld2WC9PVG5nN3FxUWExbytLYkxwVjlvUkl2UmxaS3IzU0U5cjhlc2s4Qk5zMVFkQ3g5N2V2eC96UHhGWi83UkNqSzVqZm95WUE5ZUFBYXErZHlRYVZlZ2svSWU0dEZXSUF4U0pSMmpIQ09RTkZCOCtCbEVzWWlOOWVQQjNwMXpzYlBtL0VxWHQ0eHdLMzVvci9BbndrNzVqcExZeXpzb2JaR3ZZcFVSRlo1aHBrdEJiUTY2ZVBTZ0Fma1o3YU9WOFJaUnlTT1NyZGZ4VTRPMGlzamxvbDJ1SFk3ZCtWb3lwekJkMEk5YkR3OHJkbHNXODFpc29Jb0s5TlJ3NURSQmFVOHlTbW1oUllMVUpjaC9ERkJaYXB3YUJYcHlEOHpKbk9mSHNpQ1ZycXlsQVJWamRHUEo1QmxpblBwQ29JSjBJV2xUMVZNY05JYXRtRENQWXg4SVQrTXlwT1pQdGl1WDRTRjErUzI4d1NZS0Q0MGdLVHpxZDFwZWlIRlVheFk5ek1xNm5JVDBKZEg3Z3FLa0R0RGJrbnJDckk0cXB3KzJ2TWZweWFac3NBTU1ERVhwTnZyZ1Y5eHdvaUlTQjlvTDJUbElaTGEvemdObVdPck9Fejd4TCtkdk1pYXpQRHJ2eWR4OVNnOVA0QzhpdjlJMkNTSzBrcm1uSWJ1YmUyYlhwb3ZmVlVSYVl1Y25ueGk4bzhSMGtXQ0h5THRESFdXQmpsc1N6bHlUOUVtdUYxNGFQSHBLWU9mZTFja2JrOFVIcDR2Vk4wdWlKSmxaWGkxNS82NkJ4azN3N3JmU1FyM25aQVNTVFM5c1Mxdi9DdHl1SXpPbDRCSDRtNFRIU052NjAwTHZVdVVzcVhoUDAybFRLTVluaTc1L3N6d09QNDZaSFBrWEdIMGlVeXBMQ0dsTHpTR2hNYkV3ZkRjMmJ5RExpcm9VTzIrZHNKR1ZKWXJNOHV6a1VjaFFMc3p6bjluYmxEUWt3S0lpT3VNQVRyMzFySVhjNlM3NTZiU1Y1VFFRUXRKbGUrbjc4V1ZLSEhxWEhoYUduclkwbksySlZadzlGNmRtaUtjSlpPR3pXU20wQ3hJV0lsdnFxVTNabk1DWTFLV2RzK2Vib0Q0c09yTE5PL0E3bW9ubjhYZ2hqZmNndjZaRHRtWWZaYTAyMnkzeUsyd0t2bXdDTDhjY08vYmE3WUV5ZkozWUFSVFJURUkwMjd5YkdnTjdpWEd4T0xrbUxKT3MvY1ZOaWNiZnJ2ZTlsL1RHSm5DQk5ncTJiUXFMMTJYQUhBIiwiZGF0YWtleSI6IkFRRUJBSGlqRUZYR3dGMWNpcFZPYWNHOHFSbUpvVkJQYXk4TFVVdlU4UkNWVjBYb0h3QUFBSDR3ZkFZSktvWklodmNOQVFjR29HOHdiUUlCQURCb0Jna3Foa2lHOXcwQkJ3RXdIZ1lKWUlaSUFXVURCQUV1TUJFRUROQ0h6N1JTKzJ1RWxibXREd0lCRUlBN3kyT0ZCVW5ibEpwRzFQVkJGQjJ0djIweERLMzBmeng5MnkxbE43dEZiYUJEMHEramc3NVBFSmdKUUk0N3NoUGlXYXFBRlhBbTZ5ZC8wQTg9IiwidmVyc2lvbiI6IjIiLCJ0eXBlIjoiREFUQV9LRVkiLCJleHBpcmF0aW9uIjoxNzExOTU1NTMwfQ=="
  sudo mkdir -p /sys/fs/cgroup/${CGROUP}
  sudo ctr i rm ${IMAGE_URL}
  sudo echo '+memory' | sudo tee /sys/fs/cgroup/${CGROUP_PARENT}/cgroup.subtree_control
  sudo echo '+cpu' | sudo tee  /sys/fs/cgroup/${CGROUP_PARENT}/cgroup.subtree_control
  OUTPUT_FILE="/tmp/${CGROUP_CHILD}"
  sudo ./test.sh ${CGROUP} ${OUTPUT_FILE} sudo ctr images pull --user "AWS:${TOKEN}" ${IMAGE_URL}
  ELAPSED=$(grep elapsed ${OUTPUT_FILE}| tail -n 1)
  TIME=$(cut -d" " -f 2 <<< "${ELAPSED}" | sed -e 's/s//')
  SPEED=$(sed -e 's/.*(//' -e 's/)//' <<< "${ELAPSED}" | cut -d" " -f 1)
  >&2 echo "${ELAPSED}"
  MEMORY=$(cat /sys/fs/cgroup/${CGROUP}/memory.peak)
  CPU=$(cat /sys/fs/cgroup/${CGROUP}/cpu.stat)
  echo "Parallel: ${ECR_PULL_PARALLEL},Time: ${TIME},Speed: ${SPEED},Memory: ${MEMORY}"
  echo "\"run-$j\" : {
    \"Parallel layers\": ${ECR_PULL_PARALLEL},
    \"Pull Time\": ${TIME},
    \"Speed\": ${SPEED},
    \"Memory\": $(( ${MEMORY} / 1048576 ))
  }," >> results.json
  pull_times+=("$TIME")
  speeds+=("$SPEED")
  memories+=("$((${MEMORY} / 1048576))")
  sudo rm ${OUTPUT_FILE}
  sudo rmdir /sys/fs/cgroup/${CGROUP}
done
echo "}" >> results.json
 pull_time_avg=$(echo "${pull_times[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 speed_avg=$(echo "${speeds[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 memory_avg=$(echo "${memories[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')

 echo "\"Averages\":  {
    \"Parallel layers\": ${i},
    \"Pull Time\": ${pull_time_avg},
    \"Speed\": ${speed_avg},
    \"Memory\": ${memory_avg}
 }," >> results_averages.json

 pull_times=()
 speeds=()
 memories=()
# done
echo "]" >> results.json
