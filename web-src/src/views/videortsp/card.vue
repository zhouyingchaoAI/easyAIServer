<template>
  <div class="cursor-pointer rounded-md overflow-hidden bg-white h-full min-h-[200px] relative flex flex-col">
    <div class="flex-1 flex flex-col">
      <div class="relative bg-gray-100 aspect-video w-full flex items-center justify-center">
        <PlayCircleOutlined class="text-gray-400 text-6xl" />
        <div v-if="data.status === 'running'" class="absolute top-2 right-2">
          <a-badge status="processing" text="运行中" />
        </div>
        <div v-else-if="data.status === 'error'" class="absolute top-2 right-2">
          <a-badge status="error" text="错误" />
        </div>
        <div v-else class="absolute top-2 right-2">
          <a-badge status="default" text="已停止" />
        </div>
      </div>

      <div class="text-left space-y-2 flex-1 flex flex-col justify-between">
        <div class="text-left py-2 px-2 space-y-2">
          <div class="text-sm text-gray-600 truncate font-semibold">
            {{ data.name }}
          </div>
          <div class="text-xs text-gray-500 truncate">
            {{ data.streamName }}
          </div>

          <a-space wrap size="small">
            <a-tag v-if="data.loop" color="blue" :bordered="false">循环</a-tag>
            <a-tag v-if="data.videoCodec" :bordered="false">{{ data.videoCodec }}</a-tag>
            <a-tag v-if="data.audioCodec" :bordered="false">{{ data.audioCodec }}</a-tag>
          </a-space>

          <div v-if="data.rtspUrl" class="flex items-center gap-2 text-xs text-gray-400">
            <span class="truncate flex-1">RTSP: {{ data.rtspUrl }}</span>
            <a-tooltip title="复制RTSP地址">
              <CopyOutlined class="cursor-pointer hover:text-blue-500" @click.stop="copyRtspUrl" />
            </a-tooltip>
          </div>

          <div v-if="data.error" class="text-xs text-red-500 truncate">
            错误: {{ data.error }}
          </div>
        </div>

        <div class="flex justify-end items-center space-x-2 p-2">
          <a-tooltip v-if="data.status !== 'running'" title="启动">
            <a-button type="text" @click.stop="onStart">
              <template #icon>
                <PlayCircleOutlined />
              </template>
            </a-button>
          </a-tooltip>

          <a-tooltip v-if="data.status === 'running'" title="停止">
            <a-button type="text" @click.stop="onStop">
              <template #icon>
                <PauseCircleOutlined />
              </template>
            </a-button>
          </a-tooltip>

          <a-tooltip title="编辑">
            <a-button type="text" @click.stop="onClickEdit">
              <template #icon>
                <EditOutlined />
              </template>
            </a-button>
          </a-tooltip>

          <a-popconfirm placement="topRight" title="您确定要删除这个流任务吗？" ok-text="是" cancel-text="否"
            @confirm="onClickDelete">
            <a-tooltip title="删除">
              <a-button type="text" danger>
                <template #icon>
                  <DeleteOutlined />
                </template>
              </a-button>
            </a-tooltip>
          </a-popconfirm>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { PlayCircleOutlined, PauseCircleOutlined, DeleteOutlined, EditOutlined, CopyOutlined } from '@ant-design/icons-vue';
import { copyText } from '@/utils';

const props = defineProps(["data"]);
const emit = defineEmits(["onStart", "onStop", "onEdit", "onDelete"]);

// 复制RTSP地址
const copyRtspUrl = () => {
  if (props.data.rtspUrl) {
    copyText(props.data.rtspUrl, 'RTSP地址');
  }
};

// 启动流
const onStart = () => {
  emit("onStart", props.data.id);
};

// 停止流
const onStop = () => {
  emit("onStop", props.data.id);
};

// 点击编辑
const onClickEdit = () => {
  emit("onEdit", props.data);
};

// 确认删除
const onClickDelete = () => {
  emit("onDelete", props.data.id);
};
</script>

