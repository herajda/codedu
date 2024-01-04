#pragma once

#include <queue>
#include <mutex>
#include <shared_mutex>

class Task {
  public:
    Task(/*some arguments*/ );
    
    void execute(/*some arguments*/ );

    private:
      void fetch_files_from_database(/*some arguments*/ );
};

class TaskQueue {
  public:
    TaskQueue(/*some arguments*/ );
    
    void checknonempty(/*some arguments*/ ); // every few seconds, check if the queue is empty, if not, execute the task on the top of the queue

  private:
    std::queue<Task> task_queue;
    std::shared_mutex task_queue_mutex;

    bool is_empty();
};
