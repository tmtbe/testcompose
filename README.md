# feature
* 编排功能
  * 容器编排
  * POD模式
  * 健康监测
  * POD依赖管理
  * 只读数据卷
  * 数据卷切换
* 切换数据集
* 重启指定POD
* 动态暴露POD服务端口
* POD状态监控
* 管理API
  * POST /restart 重启指定POD
  * POST /ingress 动态暴露POD端口
  * POST /switch 切换数据集
  * POST /shutdown 关闭所有服务停止编排
  * GET /info 获取当前编排容器信息
  * POST /start 手动模式下启动编排
* EventBus 端口，用于订阅/发布容器状态变更