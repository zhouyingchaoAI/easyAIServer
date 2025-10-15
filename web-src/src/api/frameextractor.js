import request from "./request";
export default {
  getConfig(){
    return request({ url: '/frame_extractor/config', method: 'get' });
  },
  updateConfig(data){
    return request({ url: '/frame_extractor/config', method: 'post', data });
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
  listSnapshots(taskId){
    return request({ url: `/frame_extractor/snapshots/${taskId}`, method: 'get' });
  },
  deleteSnapshot(taskId, path){
    return request({ url: `/frame_extractor/snapshots/${taskId}/${path}`, method: 'delete' });
  }
}


