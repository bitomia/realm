# Get the directory where this config file is located
get_filename_component(REALM_CMAKE_DIR "${CMAKE_CURRENT_LIST_FILE}" PATH)

# Compute the installation prefix relative to this file
# Config is at: ${PREFIX}/lib/cmake/Realm/RealmConfig.cmake
# So we need to go up 3 levels to get to ${PREFIX}
get_filename_component(REALM_PREFIX "${REALM_CMAKE_DIR}/../../.." ABSOLUTE)

# Find required dependencies
find_package(Threads REQUIRED)

# Determine shared library extension based on platform
if(WIN32)
    set(REALM_LIB_NAME "librealm.dll")
elseif(APPLE)
    set(REALM_LIB_NAME "librealm.dylib")
else()
    set(REALM_LIB_NAME "librealm.so")
endif()

# Define the imported library target
if(NOT TARGET Realm::Realm)
    add_library(Realm::Realm SHARED IMPORTED)

    # Set the library location
    set_target_properties(Realm::Realm PROPERTIES
        IMPORTED_LOCATION "${REALM_PREFIX}/lib/${REALM_LIB_NAME}"
        INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/include"
        INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
    )

    # Add platform-specific libraries that Go C archives typically need
    if(UNIX AND NOT APPLE)
        # Linux
        set_property(TARGET Realm::Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "m"
        )
    elseif(APPLE)
        # macOS - may need additional frameworks
        set_property(TARGET Realm::Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "-framework CoreFoundation"
        )
    endif()
endif()

# Set package variables
set(Realm_FOUND TRUE)
set(Realm_LIBRARIES Realm::Realm)
set(Realm_INCLUDE_DIRS "${REALM_PREFIX}/include")

# Verify that the library file exists
if(NOT EXISTS "${REALM_PREFIX}/lib/${REALM_LIB_NAME}")
    message(FATAL_ERROR "Realm library not found at ${REALM_PREFIX}/lib/${REALM_LIB_NAME}")
endif()

if(NOT EXISTS "${REALM_PREFIX}/include/realm.h")
    message(FATAL_ERROR "Realm header not found at ${REALM_PREFIX}/include/realm.h")
endif()
