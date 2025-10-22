import request from "./request";

export default {
  // 获取告警列表
  listAlerts(params){
    return request({
      url: '/alerts',
      method: 'get',
      params
    });
  },
  
  // 获取告警详情
  getAlert(id){
    return request({
      url: `/alerts/${id}`,
      method: 'get'
    });
  },
  
  // 删除告警
  deleteAlert(id){
    return request({
      url: `/alerts/${id}`,
      method: 'delete'
    });
  },
  
  // 批量删除告警
  batchDeleteAlerts(ids){
    return request({
      url: '/alerts/batch_delete',
      method: 'post',
      data: { ids }
    });
  },
  
  // 获取注册的算法服务
  listServices(){
    return request({
      url: '/ai_analysis/services',
      method: 'get'
    });
  },
  
  // 获取所有任务ID列表
  getTaskIds(){
    return request({
      url: '/alerts/task_ids',
      method: 'get'
    });
  }
}

