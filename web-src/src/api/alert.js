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
  
  // 获取注册的算法服务
  listServices(){
    return request({
      url: '/ai_analysis/services',
      method: 'get'
    });
  }
}

