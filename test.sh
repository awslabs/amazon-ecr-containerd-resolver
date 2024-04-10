#!/bin/bash -e
# cgexec is a little script to exec another command inside a specified cgroup
# cgexec currently only supports memory and cpuacct cgroups
set -e
echo "Running cgexec script"
CGROUP=$1
[[ -z $CGROUP ]] && exit 1
shift
[[ -z $1 ]] && exit 1
OUTPUT=$1
[[ -z $OUTPUT ]] && exit 1
shift
[[ -z $1 ]] && exit 1

echo "writing pid $$ to file /sys/fs/cgroup/${CGROUP}/cgroup.procs"
echo "running command in background"
"$@" > ${OUTPUT} 2>&1 &

command_pid="$!"
echo $command_pid | sudo tee /sys/fs/cgroup/${CGROUP}/cgroup.procs

add_child_pid() {
   local parent=$1
   for child_pid in $(pgrep -P $parent); do
           echo $child_pid | sudo tee -a /sys/fs/cgroup/${CGROUP}/cgroup.procs
           add_child_pid $child_pid
   done
}

add_child_pid $command_pid

wait $command_pid
echo "ECR pull complete"
