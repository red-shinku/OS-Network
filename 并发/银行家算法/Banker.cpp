#include "Banker.hpp"

Banker::Banker(
    int thread_nums,
    int src_nums,
    std::vector<std::vector<int>> &threads
):
    total_src(src_nums),
    available_src(src_nums)
{
    for(auto thread: threads)
    {
        PID_func_details[thread[0]] = {thread[1], 0, 0};
        threadlist.push_back(thread[0]);
    }
}

bool Banker::request(int PID, int need)
{
    std::unique_lock<std::mutex> lock(mtx);
    Details& details = PID_func_details.at(PID);
    if(need + details.now_have > details.max_request)
    {
        return false;
    }
    if(need > available_src)
    {
        details.now_need = need;
        return false;
    }
    available_src -= need;
    details.now_have += need;
    
    if(is_safe())
    {
        details.now_need = 0;
        return true;
    }
    else
    {
        details.now_need = 0;
        available_src += need;
        details.now_have -= need;
        return false;
    }
}

bool Banker::is_safe()
{
    int available = available_src;
    DetailsQueue &pqueue = threadlist;
    std::make_heap(pqueue.begin(), pqueue.end(), cmp_func);
    for(int i = 0; i < pqueue.size(); ++i)
    {
        int cur_thread = pqueue[0];
        Details &details = PID_func_details.at(cur_thread);
        if(available < details.max_request - details.now_have)
        {
            return false;
        }
        available += details.now_have;
        std::pop_heap(pqueue.begin(), pqueue.end() - i, cmp_func);
    }
    return true;
}

void Banker::update_src(int PID, int nums)
{
    std::unique_lock<std::mutex> lock(mtx);
    auto &details = PID_func_details.at(PID);
    if(nums == 0)
    {
        available_src += details.now_have;
        details.now_have = 0;
    }
    else if(nums <= details.now_have && nums > 0)
    {
        available_src += nums;
        details.now_have -= nums;
    }
    else
    {
        return;
    }
}

void Banker::add_thread(int PID, int max_request)
{
    std::unique_lock<std::mutex> lock(mtx);
    PID_func_details[PID] = {max_request, 0, 0};
    threadlist.push_back(PID);
}


void Banker::set_details(
    std::vector<int>& t,
    std::vector<int>& have
)
{
    auto t_iter = t.cbegin();
    auto have_iter = have.cbegin();
    while(t_iter != t.cend())
    {
        if(PID_func_details.count(*t_iter) != 0)
        {
            PID_func_details[*t_iter++].now_have = *have_iter;
            available_src -= *have_iter++;
        }
        else
        {
            ++t_iter;
            ++have_iter;
        }
    }
}

/*
bool Banker::test_and_remove()
{
    if(waitTORemove.size() == 0 || detailsQueue.size()) return false;

    if(waitTORemove[0] != detailsQueue[0])
    {
        return false;
    }
    if(PID_func_details.count(waitTORemove[0]) == 0)
    {
        detailsQueue.pop();
        waitTORemove.pop();
        return true;
    }
    else
    {
        waitTORemove.pop();
        return false;
    }
}
*/