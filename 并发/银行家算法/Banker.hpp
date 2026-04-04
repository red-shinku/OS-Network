#ifndef BANKER_H
#define BANKER_H

#include <algorithm>
#include <vector>
#include <map>
#include <queue>
#include <functional>
#include <mutex>

/**
 * Details：保存线程资源使用情况
 * 每个注册线程拥有一个该结构体
 * 成员变量：
 *     max_request：该线程允许的最大资源数
 *     now_have：该线程当前持有的资源
 *     now_need：该线程当前需要的资源
 */

struct Details
{
    int max_request;
    int now_have;
    int now_need;
};

/**
 * Banker类（死锁预防）
 * 在线程申请资源前，检查是否会出现死锁
 * 注册了一组使用同一批资源的线程id
 * 申请资源的线程调用request方法，若安全，
 * 返回true允许申请，否则返回false拒绝。
 * 释放资源的线程需要调用update_src方法，更新
 * 资源使用信息。
 * 该类以禁止拷贝操作，未定义移动操作
 * 请在堆内存上创建该对象。
 * 
 * 成员变量：
 *     私有：
 *     mtx：同步原语，互斥量，控制该对象的入口
 *     total_src：总的资源数
 *     available_src：当前可用资源数
 *     PID_func_details：线程id到线程Details的映射
 *     cmp_func：可调用对象，用于优先队列的比较方法
 *     threadlist：
 *           当前线程列表，存储线程ID
 * 
 * 成员函数：
 *     私有：
 *     is_safe(int PID)：
 *          检查分配资源后，线程是否安全                      
 * 
 *     对外：
 *     request(int PID, int need)：
 *          线程请求资源时调用。先预分配，检查是否安全。
 *     update_src(int PID, int nums):
 *          线程释放资源时调用。更新其资源持有信息。
 *     add_thread(int PID, int max_request)：
 *          注册新的线程id，提供对该线程的死锁检查服务。
 *     remove(K&&... pid)：
 *          取消对线程的检查服务。使用可变参数模版。
 *          同时提供对给定实参合法性的检查。若不合法则
 *          忽略。若合法，从PID_func_details中移除。
 */

class Banker 
{
public:
    Banker(int thread_nums, int src_nums, std::vector<std::vector<int>>&);
    Banker(const Banker&) = delete;
    Banker& operator=(const Banker&) = delete;
    ~Banker() = default;

private:
    using DetailsQueue = std::vector<int>;

    std::mutex mtx;
    int total_src;
    int available_src;
    std::map<int, Details> PID_func_details;

    std::function<bool(int, int)> cmp_func = 
    [this](const int& PID1, const int& PID2) -> bool{
        auto &p1 = PID_func_details.at(PID1);
        auto &p2 = PID_func_details.at(PID2); 
        return p1.max_request - p1.now_have < 
            p2.max_request - p2.now_have;
    };
    DetailsQueue threadlist;

    bool is_safe();
    //bool test_and_remove();

public:
    bool request(int PID, int need);
    void update_src(int PID, int nums = 0);
    void add_thread(int PID, int max_request);
    template<typename... K> void remove(K&&... PID);

    //test method. should be delete in real work
    void set_details(std::vector<int>&, std::vector<int>&);
    bool issafe() { return is_safe(); }
};


template<typename... K>
void Banker::remove(K&&... PID)
{
    auto remove_thread = [&](auto&& pid){
        if(PID_func_details.count(pid) == 1)
        {
            threadlist.erase(
                std::remove(
                    threadlist.begin(),
                    threadlist.end(),
                    pid
                ),
                threadlist.end()
            );
            PID_func_details.erase(pid);
        }
    };

    std::unique_lock<std::mutex> lock(mtx);
    (remove_thread(PID), ...);
}


#endif //BANKER_H