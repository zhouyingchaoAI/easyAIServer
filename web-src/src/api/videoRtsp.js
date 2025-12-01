import request from './request'

export default {
  // 获取流任务列表
  getStreamList(data) {
    return request({
      url: '/video_rtsp',
      method: 'get',
      params: data,
    })
  },

  // 获取单个流任务信息
  getStreamItem(id) {
    return request({
      url: `/video_rtsp/${id}`,
      method: 'get',
    })
  },

  // 创建流任务
  createStream(data) {
    return request({
      url: '/video_rtsp',
      method: 'post',
      data,
    })
  },

  // 更新流任务
  updateStream(id, data) {
    return request({
      url: `/video_rtsp/${id}`,
      method: 'put',
      data,
    })
  },

  // 删除流任务
  deleteStream(id) {
    return request({
      url: `/video_rtsp/${id}`,
      method: 'delete',
    })
  },

  // 启动流
  startStream(id) {
    return request({
      url: `/video_rtsp/${id}/start`,
      method: 'post',
    })
  },

  // 停止流
  stopStream(id) {
    return request({
      url: `/video_rtsp/${id}/stop`,
      method: 'post',
    })
  },

  // 获取视频文件列表
  getVideoFiles(dir) {
    return request({
      url: '/video_rtsp/files',
      method: 'get',
      params: { dir },
    })
  },
}

