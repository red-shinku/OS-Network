#include "Banker.hpp"
#include <iostream>

//test program

int main()
{
    //change these vector to test

    //thread now have and their max_request
    std::vector<std::vector<int>> PID_MAXR = {
        {1001, 11}, {1002, 9}, {1003, 9}, {1004, 5}, {1005, 8}
    };

    //thread now have and resource details
    std::vector<int> threadlist = {1001, 1002, 1003, 1004, 1005};
    std::vector<int> each_have = {1, 3, 3, 2, 2};
    
    //new a banker
    Banker *banker = new Banker(5, 14, PID_MAXR);

    //set resource each thread have
    banker->set_details(threadlist, each_have);
    
    std::cout << banker->issafe() << std::endl;
}