get_filename_component(REALM_CMAKE_DIR "${CMAKE_CURRENT_LIST_FILE}" PATH)
get_filename_component(REALM_PREFIX "${REALM_CMAKE_DIR}/.." ABSOLUTE)

find_package(Threads REQUIRED)

# Determine shared library extension based on platform
if(WIN32)
    set(REALM_LIB_NAME "librealm.dll")
    set(REALM_IMPLIB_NAME "librealm.lib")
    set(REALM_HEADER_NAME "realm.h")
else()
    set(REALM_LIB_NAME "librealm.so")
    set(REALM_HEADER_NAME "realm.h")
endif()

# Define the imported library target
if(NOT TARGET Realm)
    if(WIN32)
        add_library(Realm SHARED IMPORTED)
        set_target_properties(Realm PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_LIB_NAME}"
            IMPORTED_IMPLIB "${REALM_PREFIX}/bin/${REALM_IMPLIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    else()
        add_library(Realm SHARED IMPORTED)
        set_target_properties(Realm PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_LIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    endif()

    # Add platform-specific libraries that Go C archives typically need
    if(UNIX AND NOT APPLE)
        # Linux
        set_property(TARGET Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "m"
        )
    elseif(APPLE)
        # macOS - may need additional frameworks
        set_property(TARGET Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "-framework CoreFoundation"
        )
    endif()
endif()

# Set package variables
set(Realm_FOUND TRUE)
set(Realm_LIBRARIES Realm)
set(Realm_INCLUDE_DIRS "${REALM_PREFIX}/bin")

# Verify that the library files exist
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_LIB_NAME}")
    message(FATAL_ERROR "Realm library not found at ${REALM_PREFIX}/bin/${REALM_LIB_NAME}")
endif()
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_HEADER_NAME}")
    message(FATAL_ERROR "Realm header not found at ${REALM_PREFIX}/bin/${REALM_HEADER_NAME}")
endif()
