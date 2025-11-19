import request from "./request";

export default {
  // 获取推理统计信息
  getInferenceStats() {
    return request({
      url: '/ai_analysis/inference_stats',
      method: 'get'
    });
  },
  
  // 重置推理统计数据
  resetInferenceStats() {
    return request({
      url: '/ai_analysis/inference_stats/reset',
      method: 'post'
    });
  },
  
  // 获取算法服务列表
  listServices() {
    return request({
      url: '/ai_analysis/services',
      method: 'get'
    });
  },
  
  // 获取负载均衡信息
  getLoadBalanceInfo() {
    return request({
      url: '/ai_analysis/load_balance/info',
      method: 'get'
    });
  }
}

