#!/bin/bash -e
set -e

declare -a pull_times
declare -a speeds
declare -a memories

sudo rm results.json
sudo rm results_averages.json
sudo rm -rf ./bin
sudo make build
img="ecr.aws/arn:aws:ecr:us-west-1:020023120753:repository/3gb-single:latest"
echo $img >> results_averages.json
echo "[" >> results.json
echo "{" >> results_averages.json
for i in $(seq 0 7); do
set -e
echo "{" >> results.json
count=0
for j in $(seq 1 5); do
  >&2 echo "Run: $j with parallel arg: $i"
  ECR_PULL_PARALLEL=$(( 7 - $i))
  #ECR_PULL_PARALLEL=0
  >&2 sudo service containerd stop
  >&2 sudo rm -rf /var/lib/containerd
  >&2 sudo mkdir -p /var/lib/containerd
  >&2 sudo service containerd start
  CGROUP_PRENT="ecr-pull-benchmark"
  CGROUP_CHILD="count-$j-parallel-${ECR_PULL_PARALLEL}-slice"
  CGROUP=${CGROUP_PARENT}/${CGROUP_CHILD}
  IMAGE_URL=$img
  sudo mkdir -p /sys/fs/cgroup/${CGROUP}
  sudo echo '+memory' | sudo tee /sys/fs/cgroup/${CGROUP_PARENT}/cgroup.subtree_control
  sudo echo '+cpu' | sudo tee  /sys/fs/cgroup/${CGROUP_PARENT}/cgroup.subtree_control
  OUTPUT_FILE="/tmp/${CGROUP_CHILD}"
  sudo ./test.sh ${CGROUP} ${OUTPUT_FILE} sudo ECR_PULL_PARALLEL="${ECR_PULL_PARALLEL}" ./bin/ecr-pull ${IMAGE_URL}
  ELAPSED=$(grep elapsed ${OUTPUT_FILE}| tail -n 1)
  UNPACK=$(grep unpackTime ${OUTPUT_FILE}| tail -n 1)
  TIME=$(cut -d" " -f 2 <<< "${ELAPSED}" | sed -e 's/s//')
  UNPACKTIME=$(cut -d" " -f 2 <<< "${UNPACK}" | sed -e 's/s//')
  SPEED=$(sed -e 's/.*(//' -e 's/)//' <<< "${ELAPSED}" | cut -d" " -f 1)
  >&2 echo "${ELAPSED}"
  MEMORY=$(cat /sys/fs/cgroup/${CGROUP}/memory.peak)
  CPU=$(cat /sys/fs/cgroup/${CGROUP}/cpu.stat)
  echo "Parallel: ${ECR_PULL_PARALLEL},Time: ${TIME},Speed: ${SPEED},Memory: ${MEMORY}, Unpack: ${UNPACKTIME}"
  tot_Mem=$(( ${MEMORY} / 1048576 ))
  echo "\"run-$j\" : {
    \"Parallel layers\": ${ECR_PULL_PARALLEL},
    \"Pull Time\": ${TIME},
    \"Unpack Time\": ${UNPACKTIME},
    \"Speed\": ${SPEED},
    \"Memory\": ${tot_Mem}
  }," >> results.json
  echo "tot_Mem = ${tot_Mem}"
  if [ ${tot_Mem} -ne 0 ]
  then
   pull_times+=("$TIME")
   speeds+=("$SPEED")
   memories+=("$tot_Mem")
   unpack_times+=("$UNPACKTIME")
   count=$((count + 1))
  fi
  sudo rm ${OUTPUT_FILE}
  sudo rmdir /sys/fs/cgroup/${CGROUP}
done
 echo "}" >> results.json
 pull_time_avg=$(echo "${pull_times[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 unpack_time_avg=$(echo "${unpack_times[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 speed_avg=$(echo "${speeds[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 memory_avg=$(echo "${memories[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum/NR}')
 total_pull_time=$(echo "${pull_times[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum}')
 total_speed=$(echo "${speeds[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum}')
 total_memory=$(echo "${memories[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum}')
 total_unpack_time=$(echo "${unpack_times[@]}" | tr ' ' '\n' | awk '{sum+=$1} END {print sum}')
 echo "\"Averages\":  {
    \"Avg Parallel layers\": $(( 7 - $i)),
    \"Avg Pull Time\": ${pull_time_avg},
    \"Avg Unpack Time\": ${unpack_time_avg},
    \"Avg Speed\": ${speed_avg},
    \"Avg Memory\": ${memory_avg},
    \"Total_Download_time\": ${total_pull_time},
    \"Total_speed\": ${total_speed},
    \"Total_mem\": ${total_memory},
    \"Total_Unpack_time\": ${total_unpack_time},
    \"Count\":${count}
 }," >> results_averages.json

 pull_times=()
 speeds=()
 memories=()
 unpack_times=()
 count=0
done
  echo "]" >> results.json
  echo "=================================================================" >> results.json
