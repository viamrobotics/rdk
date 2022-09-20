#include "../vafw/encoder.h"

#include <iostream>

#define CHECK(E)                                                             \
    if (E) {                                                                 \
    } else {                                                                 \
        std::cerr << "failed " << #E << " @ " << __FILE__ << ":" << __LINE__ \
                  << std::endl;                                              \
        exit(1);                                                             \
    }

#define ASSERT(EXPRESSION) ASSERT_TRUE(EXPRESSION)

int main() {
    IncrementalEncoder e;

    CHECK(0 == e.position());

    e.encoderTick(true);  // 1->4
    CHECK(-1 == e.position());

    e.encoderTick(false);  // 4->3
    CHECK(-2 == e.position());

    e.encoderTick(true);  // 3->2
    CHECK(-3 == e.position());

    e.encoderTick(false);  // 2->1
    CHECK(-4 == e.position());

    e.encoderTick(false);  // 1->2
    CHECK(-3 == e.position());

    e.encoderTick(true);  // 2->3
    CHECK(-2 == e.position());

    e.encoderTick(false);  // 3->4
    CHECK(-1 == e.position());

    e.encoderTick(true);  // 4->1
    CHECK(0 == e.position());
}
