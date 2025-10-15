import request from "./request";
export default {
  getConfig(){
    return request({ url: '/frame_extractor/config', method: 'get' });
  },
  updateConfig(data){
    return request({ url: '/frame_extractor/config', method: 'post', data });
  },
  getTaskTypes(){
    return request({ url: '/frame_extractor/task_types', method: 'get' });
  },
  listTasks(){
    return request({ url: '/frame_extractor/tasks', method: 'get' });
  },
  addTask(data){
    return request({ url: '/frame_extractor/tasks', method: 'post', data });
  },
  delTask(id){
    return request({ url: `/frame_extractor/tasks/${id}`, method: 'delete' });
  },
  startTask(id){
    return request({ url: `/frame_extractor/tasks/${id}/start`, method: 'post' });
  },
  stopTask(id){
    return request({ url: `/frame_extractor/tasks/${id}/stop`, method: 'post' });
  },
  updateInterval(id, intervalMs){
    return request({ url: `/frame_extractor/tasks/${id}/interval`, method: 'put', data: { interval_ms: intervalMs } });
  },
  getStatus(id){
    return request({ url: `/frame_extractor/tasks/${id}/status`, method: 'get' });
  },
  listSnapshots(taskId){
    return request({ url: `/frame_extractor/snapshots/${taskId}`, method: 'get' });
  },
  deleteSnapshot(taskId, path){
    return request({ url: `/frame_extractor/snapshots/${taskId}/${path}`, method: 'delete' });
  },
  batchDeleteSnapshots(taskId, paths){
    return request({ url: `/frame_extractor/snapshots/${taskId}/batch_delete`, method: 'post', data: { paths } });
  }
}


