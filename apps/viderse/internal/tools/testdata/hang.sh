#!/bin/sh
# Script giả cho test nhóm process: in tiến độ rồi treo mãi. Một con cháu
# cũng treo và bỏ qua SIGTERM — chỉ SIGKILL cả nhóm mới dọn hết.
sh -c 'trap "" TERM; while :; do sleep 1; done' &
echo "grandchild:$!"
echo "progress:10%"
echo "progress:20%"
while :; do sleep 1; done
